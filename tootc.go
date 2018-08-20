package main

import "fmt"
import "bufio"
import "os"
import "flag"
import "strings"
import "bytes"
import "io/ioutil"
//import "strconv"
import "encoding/json"
import "unicode/utf8"
import "crypto/sha256"
import "encoding/base64"

// Globals
var config map[string] string // Map of config variables
var naughtyRunes string // Unallowed runes in user input
var maxTootRunes int // Maximum runes allowed in any toot

// Simple check func
func check(e error){
	if e != nil {
		panic(e)
	}
}

// Print string then exit(1)
func die(str string){
	fmt.Println(str)
	os.Exit(1)
}

// Simple debug func
func dbg(str string){
	fmt.Println(str)
}

// Print usage and then exit
func usage(){
	fmt.Println("tootc [-pu] [-f user@instance] [-l post_id] [-m user@instance[,...]] [-r post_id] [-cf file]")
	fmt.Println("if invoked with no arguments or -cf tootc reads toots from the user's inbox")
	fmt.Println("-u (Usage) print this message then exit")
	fmt.Println("-p (Post) read from stdin posting the content to the user's timeline")
	fmt.Println("-f (Follow) follow user@instance")
	fmt.Println("-l (Like) like post_id")
	fmt.Println("-m (Message) read from stdin messaging comma separated list of [user@instance[,...]] directly")
	fmt.Println("-r (Reply) post reply to post_id")
	fmt.Println("-cf use a different configuration file than ~/.tootc")
	fmt.Println("flmpru are all mutually exclusive")
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
	dbg("")
}

// Takes []bytes of input(utf8) and length limit
// Returns [][]bytes each limited to limit number of runes
func splitRunes(input []byte, limit int) [][]byte {
	rv := make([][]byte, 0)
	for {
		if utf8.RuneCount(input) <= limit {
			return append(rv, input)
		}
		tmp := make([]byte, 0)
		for ii := 0; ii < limit; ii++ {
			_, size := utf8.DecodeRune(input)
			for jj := 0; jj < size; jj++ {
				tmp = append(tmp, input[0])
				input = input[1:]
			}
		}
		rv = append(rv, tmp)
	}
}

// Validates ActivityPub actor IDs
// Returns true if all valid, false if not
func validateActorIDs(actorIDs []string) bool {
	for _, v := range actorIDs {
		if ! validateActorID(v) {
			return false
		}
	}
	return true
}

// Validates a single ActivityPub actor ID
// Returns true if valid, false if not
// This is incredibly simplistic and likely wrong, will update over time
func validateActorID(actorID string) bool {
	actorID = strings.TrimSpace(actorID)
	if len(actorID) > 255 {
		return false
	}
	if strings.ContainsAny(actorID, naughtyRunes) {
		return false
	}
	if strings.Count(actorID, "@") != 1 {
		return false
	}
	s := strings.Split(actorID, "@")
	if len(s[0]) + len(s[1]) < 2 {
		return false
	}
	if strings.Count(s[1], ".") < 1 {
		return false
	}
	for _, label := range strings.Split(s[1], ".") {
		if len(label) < 1 {
			return false
		}
	}
	return true
}

func composeDirectMessage(s string, actorIDs []string) string {
	msg := struct {
    Context string `json:"@context"`
		Type string `json:"type"`
		To []string `json:"to"`
    AttributedTo string `json:"attributedTo"`
		Content string `json:"content"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Note",
		To: actorIDs,
		AttributedTo: config["ActorPage"],
		Content: s }

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	return string(j)
}

func main(){
	dbg("Starting")

	// Default values
	cfgFileNameDefault := "~/.tootc"
	maxTootRunes = 500 // The 500 text-characters(runes) limit is a Mastodon limit. We are intentionally conservative.
	naughtyRunes = "`!#$%&*<>,?\\|[]{}'\";"

	// Our CLI flags
	invokeUsage := flag.Bool("u", false, "Print usage then exit")
	invokePost := flag.Bool("p", false, "Post stdin to timeline")
	invokeFollow := flag.String("f", "", "Follow user@instance")
	invokeLike := flag.String("l", "", "Like post_id")
	invokeMessage := flag.String("m", "", "Message user@instance directly from stdin")
	invokeReply := flag.String("r", "", "Reply to post_id with stdin")
	cfgFileName := flag.String("cf", cfgFileNameDefault, "Configuration file")
	flag.Parse()

	if *cfgFileName == cfgFileNameDefault {
		readConfig(os.Getenv("HOME") + strings.TrimLeft(cfgFileNameDefault, "~"))
		if flag.NFlag() > 1 {
			die("Too many CLI arguments")
		}
	}else{
		readConfig(*cfgFileName)
		if flag.NFlag() > 2 {
			die("Too many CLI arguments")
		}
	}

	// Determine why we are being invoked
	if *invokeUsage {
		usage()

	}else if *invokePost {
		dbg("Post")
		fd, err := os.Stdin.Stat()
		check(err)
		if fd.Mode() & os.ModeNamedPipe == 0 {
			die("Failed to read stdin")
		}else{
			bytes, err := ioutil.ReadAll(os.Stdin)
			check(err)
			if len(bytes) > 0 {
				dbg("data found on stdin" + string(bytes))
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
			die("Failed to read stdin")
		}else{
			stdInput, err := ioutil.ReadAll(os.Stdin)
			check(err)
			if len(stdInput) > 0 {
				dbg("data found on stdin" + string(stdInput))

				var actorIDs []string
				for _, v := range strings.Split(*invokeMessage, ","){
					actorIDs = append(actorIDs, strings.TrimSpace(v))
				}
				if validateActorIDs(actorIDs){
					if utf8.Valid(stdInput){
						for _, v := range splitRunes(bytes.TrimRight(stdInput, "\n"), maxTootRunes){
							msg := composeDirectMessage(string(v), actorIDs)
							dbg(msg)
							hash := sha256.Sum256([]byte(msg))
							outFile := strings.TrimRight(config["Outbox"], "/") + "/" + base64.RawURLEncoding.EncodeToString(hash[:]) + ".json"
							if _, err := os.Stat(outFile); err != nil {
								if fd, err := os.Create(outFile); err == nil {
									defer fd.Close()
									fd.WriteString(msg)
								}else{
									fmt.Println(" failed:", err)
									fd.Close()
									die("Error creating file")
								}
							}else{
								die("Message already queued")
							}
						}
					}else{
						die("Invalid utf8 from stdin")
					}
				}else{
					die("Invalid Actor ID(s)")
				}
			}else{
				die("Zero bytes read on stdin")
			}
		}

	}else if len(*invokeReply) > 0 {
		dbg("Reply")

	}else{ // Read toots from user's inbox
		dbg("Read")
	}

	dbg("Finished \n")
}
