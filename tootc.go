package main

import "fmt"
import "bufio"
import "os"
import "flag"
import "strings"
import "io/ioutil"
//import "strconv"
import "encoding/json" 


// Our global map of config variables
var config map[string] string

// Simple check func 
func check(e error){
	if e != nil {
		panic(e)
	}
}

// Simple debug func
func dbg(str string){
	fmt.Println(str)
}

// Print usage and then exit
func usage(){
	fmt.Println("tootc [-p] [-f user@instance] [-l post_id] [-m user@instance[,...]] [-r post_id] [-cf file]")
	fmt.Println("if invoked with no arguments or -cf tootc reads toots from the user's inbox")
	fmt.Println("-p (Post) read from stdin posting the content to the user's timeline")
	fmt.Println("-f (Follow) follow user@instance")
	fmt.Println("-l (Like) like post_id")
	fmt.Println("-m (Message) read from stdin messaging user@instance directly")
	fmt.Println("-r (Reply) post reply to post_id")
	fmt.Println("-cf use a different configuration file than ~/.tootc")
	fmt.Println("pflmr are all mutually exclusive")
	os.Exit(0)
}

// Read in config from passed filename
func readConfig(fName string){
	// Init global defaults
	config = make(map[string]string)
	config["Inbox"] = ""
	config["Outbox"] = ""
	config["ActorPage"] = ""

	fd, err := os.Open(fName)
	check(err)
	defer fd.Close()

	confScanner := bufio.NewScanner(fd)
	for confScanner.Scan(){
		l := strings.TrimSpace(confScanner.Text())

		if len(l) == 0 {
			continue
		}
		if strings.Index(l, "#") == 0 {
			continue
		}

		kv := strings.Split(l, "=")
		if len(kv) == 1 {
			continue
		}else{
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])

			if _, exists := config[k]; exists {
				switch {
				default: // Handle all single value strings here
					config[k] = v
				}
			}
		}
	}
	for k, v := range config {
		dbg("Key:" + k + " Value:" + v)
	}
}

/*
func postToJSON(msg string){
	return 42

}
*/

// Validates ActivityPub actor IDs
// Returns true if valid, false if not
func validateActorID(actorID string) bool {
	return true
}


func composeDirectMessage(s string, actorID string){

	//	TODO: Accept multiple actorIDs and truncate messages to 500 characters

	msg := struct {
    Context string `json:"@context"`
		Type string `json:"type"`
		To string `json:"to"`
    AttributedTo string `json:"attributedTo"`
		Content string `json:"content"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Note",
		To: actorID,
		AttributedTo: config["ActorPage"],
		Content: s }

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	dbg(string(j))
}

func main(){
	dbg("Starting \n")

	// Default values
	cfgFileNameDefault := "~/.tootc"

	// Our CLI flags
	invokePost := flag.Bool("p", false, "Post stdin to timeline")
	invokeFollow := flag.String("f", "", "Follow user@instance")
	invokeLike := flag.String("l", "", "Like post_id")
	invokeMessage := flag.String("m", "", "Message user@instance directly")
	invokeReply := flag.String("r", "", "Reply to post_id with stdin")
	cfgFileName := flag.String("cf", cfgFileNameDefault, "Configuration file")
	flag.Parse()

	if *cfgFileName == cfgFileNameDefault {
		readConfig(os.Getenv("HOME") + strings.TrimLeft(cfgFileNameDefault, "~"))
		if flag.NFlag() > 1 {
			dbg("Too many CLI arguments")
			usage()
		}
	}else{
		readConfig(*cfgFileName)
		if flag.NFlag() > 2 {
			dbg("Too many CLI arguments")
			usage()
		}
	}

	// Determine why we are being invoked
	if *invokePost {
		dbg("Post")
		fd, err := os.Stdin.Stat()
		check(err)
		if fd.Mode() & os.ModeNamedPipe == 0 {
			dbg("Failed to read stdin")
			os.Exit(1)
		}else{
			bytes, err := ioutil.ReadAll(os.Stdin)
			check(err)
			if len(bytes) > 0 {
				dbg("data found on stdin")
				dbg(string(bytes))
				//postToJSON(string(bytes))
			}
		}

	}else if len(*invokeFollow) > 0 {
		dbg("Follow")

	}else if len(*invokeLike) > 0 {
		dbg("Like")

	}else if len(*invokeMessage) > 0 {
		dbg("Message")
		fd, err := os.Stdin.Stat()
		check(err)
		if fd.Mode() & os.ModeNamedPipe == 0 {
			dbg("Failed to read stdin")
			os.Exit(1)
		}else{
			bytes, err := ioutil.ReadAll(os.Stdin)
			check(err)
			if len(bytes) > 0 {
				dbg("data found on stdin")
				dbg(string(bytes))
				if validateActorID(*invokeMessage){
					composeDirectMessage(strings.TrimRight(string(bytes), "\n"), *invokeMessage)
				}else{
					dbg("Invalid Actor ID")
					os.Exit(1)
				}
			}
		}

	}else if len(*invokeReply) > 0 {
		dbg("Reply")

	}else{ // Read toots from user's inbox
		dbg("Read")
	}

	dbg("Finished \n")
}
