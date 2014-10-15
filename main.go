package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/kr/pty"

	"code.google.com/p/go.crypto/ssh"
)

func serverConfig() *ssh.ServerConfig {
	privateBytes, err := ioutil.ReadFile("./nopass_rsa")
	if err != nil {
		log.Fatalf("[FATAL] Failed to load private key %q", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatalf("[FATAL] Failed to parse private key %q", err)
	}

	// FIXME: do not accepts any keys here
	conf := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	conf.AddHostKey(private)
	return conf
}

func main() {
	conf := serverConfig()

	// start the server
	addr := fmt.Sprintf("0.0.0.0:%s", os.Args[1])
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[FATAL] failed to listen to %q, err %q", addr, err)
	}
	log.Printf("[INFO] listening to %q", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("[FATAL] failed to accept tcp conn %q", err)
		}
		_, chs, reqs, err := ssh.NewServerConn(conn, conf)
		if err != nil {
			log.Printf("[ERROR] failed to open new ssh server conn %q", err)
			continue
		}
		// discard all requests sent outside of normal data
		go ssh.DiscardRequests(reqs)
		for ch := range chs {
			handleChannel(ch)
		}
	}
}

func handleChannel(ch ssh.NewChannel) {
	if ch.ChannelType() != "session" {
		log.Printf("[ERROR] unknown channel type %q", ch.ChannelType())
		ch.Reject(ssh.UnknownChannelType, "unknown channel type")
		return
	}
	channel, requests, err := ch.Accept()
	if err != nil {
		log.Printf("[ERROR] could not accept channel %q", err)
	}
	defer channel.Close()

	var master, slave *os.File
	defer func() {
		if master != nil {
			master.Close()
		}
		if slave != nil {
			slave.Close()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	go func(in <-chan *ssh.Request) {
		for req := range in {
			log.Printf("[INFO] got request %q", req.Type)
			switch req.Type {
			case "shell":
				if len(req.Payload) > 0 {
					// do not accept any extra commands for shell
					req.Reply(false, nil)
					return
				}
				req.Reply(true, nil)
			case "pty-req":
				pty, tty, err := pty.Open()
				if err != nil {
					log.Printf("[ERROR] failed to open a pty %q", err)
					return
				}
				master, slave = pty, tty

				wg.Done()
				// FIXME handle pty request
				req.Reply(true, nil)
			case "window-change":
				// FIXME handle window resize
				req.Reply(true, nil)
			}
		}
	}(requests)

	wg.Wait()
	go io.Copy(channel, master)
	go io.Copy(master, channel)
	s := bufio.NewScanner(slave)

	for {
		prompt(slave)
		if !s.Scan() {
			break
		}
		line := s.Text()
		// clear the terminal and continue
		if line == "cls" {
			fmt.Fprintf(slave, "\033[2J")
			fmt.Fprintf(slave, "\033[1;1H")
			continue
		}
		tokens := strings.Split(line, " ")
		cmd := exec.Command("docker", tokens...)
		cmd.Stdin = slave
		cmd.Stdout = slave
		cmd.Stderr = slave
		cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		log.Println("command: %#v", cmd)
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(master, "invalid docker cmd %q\n", line)
		}
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(master, "cmd %q failed\n", line)
		}
		fmt.Fprint(slave, "\r\n")
	}
}

func prompt(w io.Writer) {
	fmt.Fprint(w, "docker > ")
}
