package main

import(
    "fmt"
    "net/http"
    //"os"
    "crypto/tls"
    "strconv"
    "net/http/httputil"
    "code.google.com/p/go-uuid/uuid"
)

func main() {
    users := 1000

    for i := 1; i <= users; i++ {
    	go startAuth(i)
    }

    fmt.Println("FINISHED!")

    http.ListenAndServe(":8787", nil)
}

func startAuth(i int) {
    //fmt.Println("Starting... #"+strconv.Itoa(i))

	cmdId        := uuid.New()
	connectionId := "TEST_CID_"+strconv.Itoa(i)

	auth := `
{
	"command": "ClientAuth",
	"id": "`+cmdId+`",
	"data": {
		"connectionID": "`+connectionId+`",
		"info" : {
			"appVersion" : "2.5",
			"model" : "LG NEXUS 5", 
			"os" : "Android 5.0", 
			"pushToken": "1234567890987654321",
			"mid" : "1234567890"
		}
	}
}
`
    //fmt.Println(auth)

    cert, err := tls.LoadX509KeyPair("mag_server.crt", "mag_server.key")
    if err != nil {
        fmt.Println("FAILED LOAD CERT #"+strconv.Itoa(i))
        fmt.Println(err)
        return
    }

    tlsConfig := tls.Config{
        Certificates: []tls.Certificate{cert},
        InsecureSkipVerify: true, // COMMENT IT OUT IN PRODUCTION!
        ServerName: "mag.ndmsystems.com", // HARDCODE!
    }

	conn, err := tls.Dial("tcp", "node1.mag.ndmsystems.com:2424", &tlsConfig)
    if err != nil {
        fmt.Println("FAILED TO DIAL #"+strconv.Itoa(i))
        fmt.Println(err)
        return
    }

    chunkedWriter := httputil.NewChunkedWriter(conn)

    chunkedWriter.Write([]byte(auth))
    chunkedWriter.Close()
    conn.Write([]byte("\r\n"))

    //fmt.Println("Finished #"+strconv.Itoa(i))
}

func generateSessionID() string {
	return uuid.New()
}
