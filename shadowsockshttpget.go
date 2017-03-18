package main

import (
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

func doOneRequest(client *http.Client, uri string, buf []byte) (err error) {
	resp, err := client.Get(uri)
	if err != nil {
		fmt.Printf("GET %s error: %v\n", uri, err)
		return err
	}
	defer resp.Body.Close()
	for err == nil {
		_, err = resp.Body.Read(buf)
	}
	if err != io.EOF {
		fmt.Printf("Read %s response error: %v\n", uri, err)
	} else {
		err = nil
	}
	return
}

func get(requestNum, connid int, uri, serverAddr string, rawAddr []byte, cipher *ss.Cipher, done chan []time.Duration) {
	reqDone := 0
	reqTime := make([]time.Duration, requestNum)
	defer func() {
		done <- reqTime[:reqDone]
	}()
	tr := &http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return ss.DialWithRawAddr(rawAddr, serverAddr, cipher.Copy())
		},
		ResponseHeaderTimeout: time.Second * 5,
	}

	timeout := time.Duration(5 * time.Second)

	buf := make([]byte, 8192)
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
	for ; reqDone < requestNum; reqDone++ {
		start := time.Now()
		if err := doOneRequest(client, uri, buf); err != nil {
			return
		}
		reqTime[reqDone] = time.Now().Sub(start)

		if (reqDone+1)%1000 == 0 {
			fmt.Printf("conn %d finished %d get requests\n", connid, reqDone+1)
		}
	}
}

func TestSpeed(server, password, method, port, uri string, connectionNum, requestNum int) float64 {

	if server == "" || port == "" || password == "" {
		fmt.Printf("Usage: -s <server> -p <port> -k <password> <url>")
		os.Exit(1)
	}

	// if strings.HasPrefix(uri, "https://") {
	// 	fmt.Println("https not supported")
	// 	os.Exit(1)
	// }
	// if !strings.HasPrefix(uri, "http://") {
	// 	uri = "http://" + uri
	// }

	cipher, err := ss.NewCipher(method, password)
	if err != nil {
		fmt.Println("Error creating cipher:", err)
		os.Exit(1)
	}
	serverAddr := net.JoinHostPort(server, port)

	parsedURL, err := url.Parse(uri)
	if err != nil {
		fmt.Println("Error parsing url:", err)
		os.Exit(1)
	}
	host, _, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		host = net.JoinHostPort(parsedURL.Host, "80")
	} else {
		host = parsedURL.Host
	}
	// fmt.Println(host)
	rawAddr, err := ss.RawAddr(host)
	if err != nil {
		panic("Error getting raw address.")
	}

	done := make(chan []time.Duration)
	for i := 1; i <= connectionNum; i++ {
		go get(requestNum, i, uri, serverAddr, rawAddr, cipher, done)
	}

	// collect request finish time
	reqTime := make([]int64, connectionNum*requestNum)
	reqDone := 0
	for i := 1; i <= connectionNum; i++ {
		rt := <-done
		for _, t := range rt {
			reqTime[reqDone] = int64(t)
			reqDone++
		}
	}

	fmt.Println("number of total requests:", connectionNum*requestNum)
	fmt.Println("number of finished requests:", reqDone)
	if reqDone == 0 {
		return 0
	}

	// calculate average an standard deviation
	reqTime = reqTime[:reqDone]
	var sum int64
	for _, d := range reqTime {
		sum += d
	}
	avg := float64(sum) / float64(reqDone)

	varSum := float64(0)
	for _, d := range reqTime {
		di := math.Abs(float64(d) - avg)
		di *= di
		varSum += di
	}
	stddev := math.Sqrt(varSum / float64(reqDone))
	fmt.Println("average time per request:", time.Duration(avg))
	fmt.Println("standard deviation:", time.Duration(stddev))
	return time.Duration(avg).Seconds()
}
