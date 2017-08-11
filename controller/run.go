package controller

import (
	minio "github.com/minio/minio-go"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	saveInterval     = 30 * time.Second
	inactiveInterval = saveInterval * 2

	maxRetries = 5
)

type controller struct {
	minioClient *minio.Client
	bucket      string
	objectName  string

	readyChan        chan bool
	stopChan         chan bool
	killChan         chan bool
	saveChan         chan bool
	backupChan       chan bool
	loginChan        chan bool
	disconnectChan   chan bool
	checkPlayersChan chan bool
	playersChan      chan int

	isError func(err error) bool

	wg sync.WaitGroup
}

func Run(endpoint, accessKeyID, secretAccessKey, bucket, objectName string, useSSL bool) {
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatal(err)
	}

	c := controller{
		minioClient: minioClient,
		bucket:      bucket,
		objectName:  objectName,

		readyChan:        make(chan bool),
		stopChan:         make(chan bool),
		killChan:         make(chan bool),
		saveChan:         make(chan bool),
		backupChan:       make(chan bool),
		loginChan:        make(chan bool),
		disconnectChan:   make(chan bool),
		checkPlayersChan: make(chan bool),
		playersChan:      make(chan int),

		wg: sync.WaitGroup{},
	}

	c.runServer()
}

func (c *controller) runServer() {
	err := c.restoreBackup()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Restored world files")

	cmd := exec.Command("java", "-Xmx1024M", "-Xms1024M", "-jar", "minecraft_server.jar", "nogui")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	c.isError = func(err error) bool {
		if err != nil {
			log.Println(err)
			cmd.Process.Kill()
			// check if killChan has already been closed, close if not
			select {
			case <-c.killChan:
			default:
				close(c.killChan)
			}
			return true
		}
		return false
	}

	c.wg.Add(4)

	go c.writeToServer(stdin)
	go c.waitAndStopServer()
	go c.backupWorld()
	go c.monitorStdout(stdout)

	log.Println("Starting minecraft server...")

	err = cmd.Start()
	if c.isError(err) {
		return
	}

	err = cmd.Wait()
	if c.isError(err) {
		return
	}

	// check if killChan has already been closed, close if not
	select {
	case <-c.killChan:
	default:
		close(c.killChan)
	}

	c.wg.Wait()
}

func (c *controller) writeToServer(stdin io.WriteCloser) {
	defer c.wg.Done()
	defer stdin.Close()
	for {
		select {
		case <-c.readyChan:
			io.WriteString(stdin, "save-off\n")
		case <-c.saveChan:
			io.WriteString(stdin, "save-all\n")
		case <-c.checkPlayersChan:
			io.WriteString(stdin, "/list\n")
		case <-c.stopChan:
			io.WriteString(stdin, "/stop\n")
		case <-c.killChan:
			return
		}
	}
}

func (c *controller) waitAndStopServer() {
	defer c.wg.Done()
	inactive := false
	ttl := time.After(inactiveInterval)
	for {
		select {
		case <-c.disconnectChan:
			c.checkPlayersChan <- true
			select {
			case playerCount := <-c.playersChan:
				if playerCount <= 0 {
					inactive = true
					log.Println("There are no more players on the server, will shut down in", inactiveInterval)
				} else {
					inactive = false
				}
			}
		case <-c.loginChan:
			inactive = false
		case <-ttl:
			c.checkPlayersChan <- true
			select {
			case playerCount := <-c.playersChan:
				if playerCount <= 0 {
					if inactive {
						c.stopChan <- true
						log.Println("Shutting down the server due to inactivity")
					} else {
						inactive = true
					}
				} else {
					inactive = false
				}
			}
		case <-c.killChan:
			return
		}
		ttl = time.After(inactiveInterval)
	}
}

func (c *controller) monitorStdout(stdout io.ReadCloser) {
	defer c.wg.Done()
	defer stdout.Close()
	buf := make([]byte, 200)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			output := string(buf[0:n])
			log.Print(output)
			switch {
			case strings.Contains(output, "[Server thread/INFO]: Done"):
				c.readyChan <- true
			case strings.Contains(output, "joined the game"):
				c.loginChan <- true
			case strings.Contains(output, "Disconnected"):
				c.disconnectChan <- true
			case strings.Contains(output, "players online"):
				for _, word := range strings.Fields(output) {
					if strings.Contains(word, "/") {
						if playerCount, err := strconv.Atoi(strings.Split(word, "/")[0]); err == nil {
							c.playersChan <- playerCount
						}
					}
				}
			case strings.Contains(output, "[Server thread/INFO]: Saved the world"):
				c.backupChan <- true
			default:
			}
		}
		select {
		case <-c.killChan:
			return
		default:
			if c.isError(err) {
				return
			}
		}
	}
}

func (c *controller) backupWorld() {
	defer c.wg.Done()
	autoSave := time.Tick(saveInterval)
	for {
		select {
		case <-autoSave:
			c.saveChan <- true
		case <-c.backupChan:
			c.createBackup(maxRetries)
		case <-c.killChan:
			c.createBackup(maxRetries)
			return
		}
	}
}
