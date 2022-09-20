package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	serial "go.bug.st/serial.v1"
)

func main() {
	portName, err := findPort()
	if err != nil {
		log.Fatal(err)
	}
	port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	fmt.Println("get status")
	{
		res, err := SendRequest(port, RequestInquire)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("got status: %q\n", res)
	}
	time.Sleep(2 * time.Second)

	fmt.Println("turn off")
	{
		res, err := SendRequest(port, RequestTurnOff)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("got response: %q\n", res)
	}
	time.Sleep(2 * time.Second)

	fmt.Println("turn on")
	{
		res, err := SendRequest(port, RequestTurnOn)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("got response: %q\n", res)
	}
}

func findPort() (string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return "", fmt.Errorf("get ports list: %w", err)
	}
	for _, port := range ports {
		fmt.Printf("Found port: %#v\n", port)
		if strings.Contains(port, "usbmodem002E1E6204511") {
			return port, nil
		}
	}
	return "", errors.New("the target port is not found")
}

const (
	Prefix      = "PW="
	Suffix      = "\r\n"
	ReadTimeout = 5 * time.Second
)

type Request int

const (
	RequestTurnOff Request = iota
	RequestTurnOn
	RequestInquire
)

func (r Request) String() string {
	switch r {
	case RequestTurnOff:
		return "0"
	case RequestTurnOn:
		return "1"
	}
	return "?"
}

type Response int

const (
	ResponseOff Response = iota
	ResponseOn
	ResponseOk
)

func (r Response) String() string {
	switch r {
	case ResponseOff:
		return "off"
	case ResponseOn:
		return "on"
	case ResponseOk:
		return "ok"
	}
	return ""
}

func ParseResponse(s string) (Response, error) {
	switch strings.TrimRight(s, "\x00O") {
	case Prefix + "0":
		return ResponseOff, nil
	case Prefix + "1":
		return ResponseOn, nil
	case "OK":
		return ResponseOk, nil
	case "ERROR":
		return 0, errors.New("got error response")
	}
	return 0, fmt.Errorf("invalid response: %q", s)
}

type TimeoutReader struct {
	source   io.Reader
	duration time.Duration
}

func (r TimeoutReader) Read(p []byte) (n int, err error) {
	ch := make(chan struct{}, 1)
	fmt.Println("reading")
	defer fmt.Println("read")
	go func() {
		n, err = r.source.Read(p)
		ch <- struct{}{}
	}()
	select {
	case <-ch:
		return
	case <-time.After(r.duration):
		return 0, io.EOF
	}
}

func SendRequest(p serial.Port, r Request) (responses []Response, _ error) {
	m := Prefix + r.String() + Suffix
	if _, err := fmt.Fprint(p, m); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	s := bufio.NewScanner(TimeoutReader{
		source:   p,
		duration: ReadTimeout,
	})
	for i := 0; s.Scan(); i++ {
		res, err := ParseResponse(s.Text())
		if err != nil {
			return nil, fmt.Errorf("parse response at %d: %w", i, err)
		}
		responses = append(responses, res)
	}
	return
}
