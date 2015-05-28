package main

import(
    "fmt"
    "strconv"
    "sync"
    "net"
    "strings"
    "time"
    "bufio"
    "os"
    "io"
    "sort"
    "code.google.com/p/go-uuid/uuid"
)

type cmdStat struct {
	TimeStamp int64
	ResponseTime int64
    ResponseCode int
}

type ByResponseTime []*cmdStat
func (a ByResponseTime) Len() int           { return len(a) }
func (a ByResponseTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByResponseTime) Less(i, j int) bool { return a[i].ResponseTime < a[j].ResponseTime }

type ResponseTimeMedian []int64
func (a ResponseTimeMedian) Len() int           { return len(a) }
func (a ResponseTimeMedian) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ResponseTimeMedian) Less(i, j int) bool { return a[i] < a[j] }

var wg sync.WaitGroup
var cmdStats   []*cmdStat
var usersStats []int64
var fail    = 0
var success = 0
var responseMax int64 = 0
var responseAvg int64 = 0
var responseMin int64 = 99999

func main() {

    users := 500
    start := time.Now().UnixNano()

    // Increment the WaitGroup counter.
    wg.Add(users)

    for i := 1; i <= users; i++ {
        // Create a connection
    	go startAuth(i)
    }

    // Wait for all routines to complete.
    wg.Wait()

    fmt.Println("FINISHED for " + strconv.FormatInt((time.Now().UnixNano() - start)/1000000000, 10) + "s")
    fmt.Println("Saving Report...")

    saveReport()

    fmt.Println("DONE!")
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
}`

	conn, err := net.Dial("tcp", "node1.mag.ndmsystems.com:2424")
    if err != nil {
        fmt.Println("FAILED TO DIAL #"+strconv.Itoa(i))
        fmt.Println(err)
        fail++
        return
    }

    // inc amount of connections
    start := time.Now().UnixNano()
    usersStats = append(usersStats, start)

    // send CMD
    conn.Write([]byte(strings.Replace(auth, "\n", "", -1)+"\n"))

    for {
    	line, err := bufio.NewReader(conn).ReadBytes('\n')

    	if err != nil {
            if err != io.EOF {
                // fail
                fmt.Println("FAILED TO READ: "+err.Error())
                fail++
            }
    		break
		}

		if strings.Contains(string(line), "Error") {
			// Error
			fail++
            cmdStats = append(cmdStats, &cmdStat{ResponseTime: (time.Now().UnixNano()-start), TimeStamp: start, ResponseCode: 500})
			break
		}

		if strings.Contains(string(line), "Result") {
			// Success
			cmdStats = append(cmdStats, &cmdStat{ResponseTime: (time.Now().UnixNano()-start), TimeStamp: start, ResponseCode: 200})
			success++
			break
		}
    }

    // Decrement the counter when the goroutine completes.
    wg.Done()

    //fmt.Println("Finished #"+strconv.Itoa(i))
}

func generateSessionID() string {
	return uuid.New()
}

func saveReport() {
    //var line string
    var currTime int64 = 0

    /**
     * Response times
     */
    cf, _ := os.Create("median.js")
    defer cf.Close()

    cf.Write([]byte("var medianStats = {\n"))
    var median []int64
    responseLen := 0
    for i := 0; i < len(cmdStats); i++ {
        if currTime != cmdStats[i].TimeStamp/1000000 || i == (len(cmdStats)-1) {
            //fmt.Println(currTime)
            //fmt.Println(currTime != 0)
            if currTime != 0 && len(median) > 0 {
                // write previous one

                // sort median
                sort.Sort(ResponseTimeMedian(median))

                // calc median
                responseLen = len(median)
                //fmt.Println(responseLen)
                if responseLen % 2 == 0 {
                    // even
                    responseAvg = (median[responseLen / 2] + median[(responseLen / 2)-1]) / 2
                } else {
                    // odd
                    responseAvg = median[(responseLen / 2)]
                }

                // write...
                cf.Write([]byte("\"key_"+strconv.FormatInt(currTime, 10)+"\": "+strconv.FormatInt(responseAvg/1000000, 10)+",\n"));
            }
            currTime = cmdStats[i].TimeStamp/1000000
            median   = nil
        } else {
            //fmt.Println("append RT: " + strconv.FormatInt(cmdStats[i].ResponseTime ,10))
            median = append(median, cmdStats[i].ResponseTime)
        }
    }
    cf.Write([]byte("\n}"))

    /**
     * Number of Users
     */
    uf, _ := os.Create("users.js")
    defer uf.Close()

    timeNum  := 0
    currTime  = 0
    uf.Write([]byte("var usersStats = [\n"))
    for i := 0; i < len(usersStats); i++ {
        if currTime != usersStats[i]/1000000 || i == (len(cmdStats)-1) {
            if currTime != 0 {
                // write previous one
                uf.Write([]byte("["+strconv.FormatInt(currTime, 10)+","+strconv.Itoa(timeNum)+"],\n"));
            }
            currTime = usersStats[i]/1000000
            timeNum  = 1
        } else {
            timeNum++
        }
    }
    uf.Write([]byte("\n]"))

    //fmt.Println("Min response time: " + strconv.FormatInt(responseMin/1000000, 10))
    //fmt.Println("Avg response time: " + strconv.FormatInt(responseAvg/1000000, 10))
    //fmt.Println("Max response time: " + strconv.FormatInt(responseMax/1000000, 10))
}
