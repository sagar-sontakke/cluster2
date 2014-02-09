/*
 * Written by: Sagar Sontakke
 * Description: This is a test script that tests the functionalities of cluster interface that is
 * provided. Here we run the multiple servers that are specified in the cluster-config file. Then we send
 * multiple messages among these servers and test whether they are getting correctly sent/received.
 */

package cluster

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

/*
 * Constant values for the test loops.
 *	1. NUMBER_LOOPS: How many times to retry the loop
 *	2. TEST_TYPE: Whether the messages are broadcasted or point-to-point or both
 *	3. MSG_DELAY: Depaly after every 10th message in milliseconds
 *	4. TIMEOUT: Timeout after n seconds
 */

const (
	TEST_TYPE     = "BOTH"            /* "BROADCAST", "UNICAST" OR "BOTH" */
	NUMBER_LOOPS  = 100               /* Number of loops to repeat (for more messages) */
	MSG_DELAY     = 1                 /* in milliseconds */
	TIMEOUT       = 1000              /* in seconds */
	CONFIG_FILE   = "clusterconf.txt" /* the cluster configuration file */
	PRINT_DETAILS = false             /* whether to print the test details at the end */
)

/* 
	Variables to record message flows
*/

var BROADCAST_MSGS bool = true
var totalsent int64 = 0
var totalreceived int64 = 0
var totalexpected int64 = -1
var WAIT_TILL int64
var totalServers int
var counter int = 0

func Test_cluster(t *testing.T) {

	start := time.Now()

	// Set up a new cluster
	servers := NewCluster(CONFIG_FILE)

	totalServers = len(servers)

	// Initialize a wait group for threads
	wg := new(sync.WaitGroup)
	wg.Add(totalServers * 3)

	// Count the expected messages to be sent and recieved
	if TEST_TYPE == "BOTH" {
		totalexpected = int64((NUMBER_LOOPS * totalServers * totalServers))
		totalexpected = totalexpected + int64(NUMBER_LOOPS*totalServers)
	} else if TEST_TYPE == "BROADCAST" {
		totalexpected = int64((NUMBER_LOOPS * totalServers * totalServers))
	} else {
		totalexpected = int64(NUMBER_LOOPS * totalServers)
	}

	WAIT_TILL = (int64(totalexpected/10) * MSG_DELAY)
	WAIT_TILL = int64(WAIT_TILL/1000) + TIMEOUT

	/*	Run the go-routines for send/receive the messages
	 */

	for i := 0; i < totalServers; i++ {
		go ReceiveMessages(servers[i])
		go SendMessages(servers[i])
	}

	/*	Loops for the test messages and send messages
	 */

	for {
		if counter > 1 {
			break
		}

		if TEST_TYPE == "BOTH" {
			BROADCAST_MSGS = !BROADCAST_MSGS
			counter = counter + 1
		} else if TEST_TYPE == "BROADCAST" {
			BROADCAST_MSGS = true
			counter = counter + 2
		} else if TEST_TYPE == "UNICAST" {
			BROADCAST_MSGS = false
			counter = counter + 2
		}

		for i := 0; i < NUMBER_LOOPS; i++ {
			for k := 0; k < totalServers; k++ {
				pid := k
				if !BROADCAST_MSGS {
					//fmt.Println("Unicasting messages")
					if k+1 >= totalServers {
						pid = servers[0].ServerId
					} else {
						pid = servers[k+1].ServerId
					}
				} else {
					//fmt.Println("Broadcasting messages")
					pid = BROADCAST
				}

				msgid := servers[k].MessageId
				msgtext := strconv.Itoa(servers[k].ServerId)

				if pid != servers[k].ServerId {

					servers[k].Outbox() <- &Envelope{pid, msgid, msgtext}

					if pid == BROADCAST {
						atomic.AddInt64(&totalsent, int64(totalServers))
					} else {
						atomic.AddInt64(&totalsent, 1)
					}

					if (totalsent % 10) == 0 {
						time.Sleep(MSG_DELAY * time.Millisecond)
					}
				}
			}

		}
	}

	for {
		for i := 0; i < totalServers; i++ {
			select {
			case env := <-servers[i].Inbox():
				if env.Pid == servers[i].ServerId {
					atomic.AddInt64(&totalreceived, 1)
				}

				if (totalreceived == totalsent) && (totalreceived == totalexpected) {
					fmt.Println("PASS")
					if PRINT_DETAILS {
						print_test_details(true)
					}
					os.Exit(0)
				}

			case <-time.After(1 * time.Millisecond):
				e := time.Since(start)
				elapsed := float64(e) / math.Pow10(9)
				if elapsed > float64(WAIT_TILL) {
					fmt.Println("FAIL")
					if PRINT_DETAILS {
						print_test_details(false)
					}
					os.Exit(1)
				} else if int64(elapsed)%3 == 0 {
					fmt.Println("sent:", totalsent, " received:", totalreceived, "   --- timeout after", TIMEOUT, "secs")
				}
				//fmt.Println("End:",elapsed)//strconv.Itoa(int(elapsed)))//time.Now().Sub(start))
			}
		}
	}

	wg.Wait()

}

/*	Print the whole test summary if the PRINT_TEST id set true
 */

func print_test_details(result bool) {
	fmt.Println("----------------------------------------------------")
	fmt.Println("Total number of servers in cluster	- ", totalServers)
	if TEST_TYPE == "BROADCAST" {
		fmt.Println("Test type				-  Broadcast")
	} else if TEST_TYPE == "UNICAST" {
		fmt.Println("Test type				-  Unicast")
	} else {
		fmt.Println("Test type				-  Broadcast and unicast")
	}
	fmt.Println("Total number of expected messages	- ", totalexpected)
	fmt.Println("Total number of sent messages		- ", totalsent)
	fmt.Println("Total number of received messages	- ", totalreceived)
	fmt.Println("----------------------------------------------------")
	if result {
		fmt.Println("Test result				-  PASS")
	} else {
		fmt.Println("Test result				-  FAIL (timed out)")
	}
	fmt.Println("----------------------------------------------------")
}
