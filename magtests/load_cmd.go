package main

import(
    "fmt"
    "strconv"
    "sync"
    "net"
    "strings"
    "bufio"
    "os"
    "io"
    "sort"
    "time"
    "math/rand"
    "code.google.com/p/go-uuid/uuid"
)

type cmdStat struct {
	TimeStamp int64
	ResponseTime int64
    ResponseCode int
}

type cmdStatAgregated struct {
    Sum int64
    Responses []int64
}

type StatsDumper struct {
    shouldStop bool
    isRunning bool
    interval int64
}
func (d StatsDumper) SetInterval(i int64) StatsDumper {
    d.interval = i
    return d
}
func (d StatsDumper) Start() StatsDumper {
    if d.interval == 0 {
        d.interval = 2 // default value, in seconds
    }

    d.shouldStop = false
    d.isRunning  = true

    go func() {
        for {
            if d.shouldStop {
                d.shouldStop = false
                d.isRunning  = false
                break
            }

            saveReport()

            wait(d.interval)
        }
    }()

    return d
}
func (d StatsDumper) Stop() {
    d.shouldStop = true
}

type UsersDumper struct {
    shouldStop bool
    isRunning bool
}
func (d UsersDumper) Start() UsersDumper {
    d.shouldStop = false
    d.isRunning  = true

    go func() {
        for {
            if d.shouldStop {
                d.shouldStop = false
                d.isRunning  = false
                break
            }

            usersStats[time.Now().Unix()] = usersCounter

            wait(1) // wait for a second...
        }
    }()

    return d
}
func (d UsersDumper) Stop() {
    d.shouldStop = true
}

type ResponseTimeMedian []int64
func (a ResponseTimeMedian) Len() int           { return len(a) }
func (a ResponseTimeMedian) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ResponseTimeMedian) Less(i, j int) bool { return a[i] < a[j] }

var wg sync.WaitGroup
var cmdStats   []*cmdStat
var usersStats = make(map[int64]int)
var rpsStats   []int64
var usersCounter = 0
var fail    = 0
var success = 0

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")


func main() {
    usersPerIteration := 30
    var iterationDuration int64 = 1 // in seconds
    var duration int64 = 300 // in seconds

    start := time.Now().Unix()

    statsDumper := new (StatsDumper).SetInterval(3).Start()
    usersDumper := new (UsersDumper).Start()

    var j = 0
    for {
        // Increment the WaitGroup counter
        wg.Add(usersPerIteration)
        for i := 0; i < usersPerIteration; i++ {
            // Create a connection
        	go startAuth(j)
            j++
        }

        // sleep for a while
        wait(iterationDuration)

        // check if we should stop
        //fmt.Println(time.Now().Unix(), start, duration, time.Now().Unix() - start)
        if (time.Now().Unix() - start) >= duration {
            fmt.Println("!!!")
            fmt.Println("!!!")
            fmt.Println("!!! Exiting as duration is over !!!")
            fmt.Println("!!!")
            fmt.Println("!!!")
            break
        }
    }

    // Wait for all routines to complete.
    wg.Wait()

    fmt.Println("FINISHED for " + strconv.FormatInt(time.Now().Unix() - start, 10) + "s")
    fmt.Println("Saving Report...")

    usersDumper.Stop()
    statsDumper.Stop()
    saveReport()

    fmt.Println("DONE!")
}

func startAuth(i int) {
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

    cmdMsg := generateRandText(500) // 500 byte of randon text
    cmd    := `
{
    "command": "CMD",
    "id": "`+cmdId+`_CMD",
    "data": {
        "connectionID": "`+connectionId+`",
        "msg" : "`+cmdMsg+`",
        "format" : "xml", 
        "encryption" : null, 
        "compression": null
    }
}`

	conn, err := net.Dial("tcp", "node1.mag.ndmsystems.com:2424")
	//conn, err := net.Dial("tcp", "localhost:2424")
    if err != nil {
        fmt.Println("FAILED TO DIAL #"+strconv.Itoa(i))
        fmt.Println(err)
        fail++
        return
    }
    defer func() {
        usersCounter--
        fmt.Println("Closing connection #"+strconv.Itoa(i))
        conn.Close()
        // Decrement the counter when the goroutine completes.
        wg.Done()
    }()

    // inc amount of connections
    start := time.Now().UnixNano()
    usersCounter++

    // send Auth
    conn.Write([]byte(strings.Replace(auth, "\n", "", -1)+"\n"))

    // write rps
    rpsStats = append(rpsStats, time.Now().Unix())

    line, err := bufio.NewReader(conn).ReadBytes('\r')
    fmt.Println("Read line from Server: " + string(line))

    // send CMD in a Loop
    for i := 0; i < 90; i++ {

        fmt.Println("Start sending CMD")

        // send CMD
        start = time.Now().UnixNano()
        conn.Write([]byte(strings.Replace(cmd, "\n", "", -1)+"\n"))

        // write rps
        rpsStats = append(rpsStats, time.Now().Unix())

        for {
        	line, err := bufio.NewReader(conn).ReadBytes('\r')
            fmt.Println("Read line from Server: " + string(line))

        	if err != nil {
                fmt.Println("ERROR: "+err.Error())
                if err != io.EOF {
                    // fail
                    fmt.Println("FAILED TO READ: "+err.Error())
                    fail++
                    return
                }
        		break
    		}

            // check if this previous Auth Result
            if strings.Contains(string(line), "ClientAuthResult") {
                continue
            }

            // check any message with this parent id
    		if strings.Contains(string(line), "_CMD") {
                fmt.Println("Log cmd stats...")
                if strings.Contains(string(line), "Error") {
        			// Error
        			fail++
                    cmdStats = append(cmdStats, &cmdStat{ResponseTime: (time.Now().UnixNano()-start), TimeStamp: start, ResponseCode: 500})
                } else /*if strings.Contains(string(line), "CMDResult")*/ {
                    // Success
                    cmdStats = append(cmdStats, &cmdStat{ResponseTime: (time.Now().UnixNano()-start), TimeStamp: start, ResponseCode: 200})
                    success++
                }
    			break
    		}
        }

        wait(1)
    }

    //fmt.Println("Finished #"+strconv.Itoa(i))
}

func generateSessionID() string {
	return uuid.New()
}

func saveReport() {
    //fmt.Println(len(cmdStats))
    //fmt.Println(len(usersStats))

    /**
     * Response times
     */
    tsMin := int64(9999999999)
    tsMax := int64(0)
    responseMin := int64(9999999999)
    responseMax := int64(0)

    cmdStatsAgregated := make(map[int64]*cmdStatAgregated)
    for i := 0; i < len(cmdStats); i++ {
        cmdStatsTS := cmdStats[i].TimeStamp / 1000000000
        if _, exists := cmdStatsAgregated[cmdStatsTS]; !exists {
            cmdStatsAgregated[cmdStatsTS] = new(cmdStatAgregated)
        }
        cmdStatsAgregated[cmdStatsTS].Sum += cmdStats[i].ResponseTime
        cmdStatsAgregated[cmdStatsTS].Responses = append(cmdStatsAgregated[cmdStatsTS].Responses, cmdStats[i].ResponseTime)

        if cmdStatsTS < tsMin {
            tsMin = cmdStatsTS
        }
        if cmdStatsTS > tsMax {
            tsMax = cmdStatsTS
        }

        if cmdStats[i].ResponseTime < responseMin {
            responseMin = cmdStats[i].ResponseTime
        }
        if cmdStats[i].ResponseTime > responseMax {
            responseMax = cmdStats[i].ResponseTime
        }
    }
    //fmt.Println(cmdStatsAgregated)

    // median file
    medianf, _ := os.Create("median.js")
    defer medianf.Close()
    // avg file
    avgf, _ := os.Create("avg.js")
    defer avgf.Close()
    // file headers...
    medianf.Write([]byte("var medianStats = {\n"))
    avgf.Write([]byte("var avgStats = {\n"))
    for ts, cmdStat := range cmdStatsAgregated {
        responsesLen := len(cmdStat.Responses)

        // calc avg
        avg := cmdStat.Sum / int64(responsesLen)

        // sort median
        sort.Sort(ResponseTimeMedian(cmdStat.Responses))

        // calc median
        responseMedian := int64(0)
        if responsesLen % 2 == 0 {
            // even
            responseMedian = (cmdStat.Responses[responsesLen / 2] + cmdStat.Responses[(responsesLen / 2)-1]) / 2
        } else {
            // odd
            responseMedian = cmdStat.Responses[(responsesLen / 2)]
        }

        // write...
        medianf.Write([]byte("\"key_"+strconv.FormatInt(ts, 10)+"\": "+strconv.FormatInt(responseMedian/1000000, 10)+",\n"));
        avgf.Write([]byte("\"key_"+strconv.FormatInt(ts, 10)+"\": "+strconv.FormatInt(avg/1000000, 10)+",\n"));
    }
    // file footers...
    medianf.Write([]byte("\n}"))
    avgf.Write([]byte("\n}"))

    /**
     * Number of Users
     */
    uf, _ := os.Create("users.js")
    defer uf.Close()
    uf.Write([]byte("var usersStats = {\n"))
    for ts, usersAmount := range usersStats {
        uf.Write([]byte("\"key_"+strconv.FormatInt(ts, 10)+"\": "+strconv.Itoa(usersAmount)+",\n"));
    }
    uf.Write([]byte("\n}"))

    uf.Write([]byte("\n"))
    uf.Write([]byte("var tsMin = " + strconv.FormatInt(tsMin, 10) + "\n"))
    uf.Write([]byte("var tsMax = " + strconv.FormatInt(tsMax, 10) + "\n"))
    uf.Write([]byte("var responseMin = " + strconv.FormatInt(responseMin/1000000, 10) + "\n"))
    uf.Write([]byte("var responseMax = " + strconv.FormatInt(responseMax/1000000, 10) + "\n"))

    //fmt.Println(tsMin, tsMax)

    /**
     * RPS
     */
    rpsStatsAgregated := make(map[int64]int)
    for i := 0; i < len(rpsStats); i++ {
        if _, exists := rpsStatsAgregated[rpsStats[i]]; !exists {
            rpsStatsAgregated[rpsStats[i]] = 0
        }
        rpsStatsAgregated[rpsStats[i]]++
    }
    //fmt.Println(rpsStatsAgregated)

    rf, _ := os.Create("rps.js")
    defer uf.Close()
    rf.Write([]byte("var rpsStats = {\n"))
    for ts, rps := range rpsStatsAgregated {
        rf.Write([]byte("\"key_"+strconv.FormatInt(ts, 10)+"\": "+strconv.Itoa(rps)+",\n"));
    }
    rf.Write([]byte("\n}"))
}

func generateRandText(n int) string {
    b := make([]rune, n)
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    for i := range b {
        b[i] = letters[r.Intn(len(letters))]
    }
    return string(b)
}

func wait(s int64) {
    start := time.Now().Unix()
    for {
        if time.Now().Unix() - start >= s {
            break
        }
    }
}
