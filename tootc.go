package main

import "fmt"
import "os"
import "errors"
import "flag"
import "strings"
import "bytes"
import "io/ioutil"
//import "strconv"
import "encoding/json"
import "unicode/utf8"
import "crypto/sha256"
import "encoding/base64"
import "net/url"

// Globals
var config map[string] map[string] string // Map of maps of config variables
var activeAccount map[string] string // Our active account config
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
	fmt.Println("tootc [-pu] [-f user@instance] [-l post_id] [-m user@instance[,...]] [-r user@instance post_id] [-cf file]")
	fmt.Println("if invoked with no arguments or -c tootc reads toots from the user's inbox")
	fmt.Println("-u (Usage) print this message then exit")
	fmt.Println("-p (Post) read from stdin posting the content to the user's timeline")
	fmt.Println("-f (Follow) follow user@instance")
	fmt.Println("-l (Like) like post_id from user@instance")
	fmt.Println("-m (Message) read from stdin messaging comma separated list of [user@instance[,...]] directly")
	fmt.Println("-r (Reply) read from stdin replying to post_id from user@instance")
	fmt.Println("-c use a different configuration file than ~/.tootc")
	fmt.Println("flmpru are all mutually exclusive")
	os.Exit(0)
}

// Read in config from passed filename
func readConfig(fName string){
	parseLine := func(line string) (string, string) {
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			return "", ""
		}
		if strings.Index(line, "#") == 0 {
			return "", ""
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			return "", ""
		}else{
			return strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		}
	}

	config = make(map[string] map[string]string)
	c, err := ioutil.ReadFile(fName)
	check(err)

	sections := strings.Split(string(c), "Account ")
	config["global"] = make(map[string]string)
	k, v := "", ""
	for _, line := range strings.Split(sections[0], "\n") {
		k, v = parseLine(line)
		if len(k) > 0 {
			config["global"][k] = v
		}
	}

	name := ""
	for _, account := range sections[1:] {
		name = strings.TrimSpace(strings.SplitN(account, "{", 2)[0]) 
		config[name] = make(map[string]string)
		for _, line := range strings.Split(account, "\n") {
			k, v = parseLine(line)
			if len(k) > 0 {
				config[name][k] = v
			}
		}
	}
	/*
	for k, v := range config {
		dbg("Key:" + k)
		for j, i := range v {
			dbg("KeyKey:" + j + " Val:" + i)
		}
	}
  */
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

// Validates an RFC822 style actor ID
// Returns true if valid, false if not
// This is incredibly simplistic and likely wrong, will update over time
func validate822(actorID string) bool {
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

// Validates a URI
// Returns true if valid, false if not
// This is incredibly simplistic and likely wrong, will update over time
func validateURI(actorID string) bool {
	u, err := url.Parse(actorID)
	if err != nil{
		return false
	}
	// Path can be nil and APub requires HTTPS
	if u.Scheme != "https" || u.Host == "" {
		return false
	}
	return true
}

// Takes user input of actor IDs
// Returns list of actorIDs as URIs for the ones it can figure out
// Returned list may be zero-length
func expandActorIDs(actorIDs []string) []string {
	var rv []string
	var err error
	actorID := ""
	for _, v := range actorIDs {
		actorID, err = expandActorID(v)
		if err == nil {
			rv = append(rv, actorID)
		}
	}
	return rv
}

// actorIDs can take these forms {user, user@domain, URI}
func expandActorID(actorID string) (string, error) {
	actorID = strings.TrimSpace(actorID)
	if strings.Contains(actorID, "@") {
		if ! validate822(actorID) {
			return "", errors.New("Invalid 822ActorID")
		}else{
			tmp := strings.Split(actorID, "@")
			user, domain := tmp[0], tmp[1]
			for _, v := range config {
				if domain == v["Domain"] {
					dbg(domain)
					if len(v["UserPrefixURI"]) > 0 {
						return v["UserPrefixURI"] + user, nil
					}else{
						return "", errors.New("UserPrefixURI not set for " + domain)
					}
				}else{
					return "", errors.New("Unknown domain " + domain)
				}
			}
		}
	}else if strings.Contains(actorID, "://") {
		if ! validateURI(actorID) {
			return "", errors.New("Invalid URI")
		}else{
			return actorID, nil
		}
	}else{
		if strings.ContainsAny(actorID, naughtyRunes) {
			return "", errors.New("Invalid UserActorID")
		}else{
			return activeAccount["UserPrefixURI"] + actorID, nil
		}
	}
	panic("Assert in expandActorID") // Won't compile without this statement :)
}

func composeNote(s string, actorIDs []string) string {
	msg := struct {
		Context string `json:"@context"`
		Type string `json:"type"`
		To []string `json:"to"`
		AttributedTo string `json:"attributedTo"`
		Content string `json:"content"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Note",
		To: actorIDs,
		AttributedTo: activeAccount["UserPrefixURI"] + activeAccount["User"],
		Content: s }

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	return string(j)
}

func composeReply(s string, actorID string, postID string) string {
	msg := struct {
		Context string `json:"@context"`
		Type string `json:"type"`
		To string `json:"to"`
		AttributedTo string `json:"attributedTo"`
		InReplyTo string  `json:"inReplyTo"`
		Content string `json:"content"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Note",
		To: actorID,
		AttributedTo: activeAccount["UserPrefixURI"] + activeAccount["User"],
		InReplyTo: postID,
		Content: s }

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	return string(j)
}

func composePost(s string) string {
	msg := struct {
		Context string `json:"@context"`
		Type string `json:"type"`
		AttributedTo string `json:"attributedTo"`
		Content string `json:"content"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Note",
		AttributedTo: activeAccount["UserPrefixURI"] + activeAccount["User"],
		Content: s }

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	return string(j)
}

func composeLike(s string, to string, postID string) string {
	msg := struct {
		Context string `json:"@context"`
		Type string `json:"type"`
		To string `json:"to"`
		Actor string `json:"actor"`
		Object string `json:"object"`
	}{ Context: "https://www.w3.org/ns/activitystreams",
		Type: "Like",
		To: to,
		Actor: activeAccount["UserPrefixURI"] + activeAccount["User"],
		Object: postID	}

	j, e := json.MarshalIndent(&msg, "", "\t")
	check(e)
	return string(j)
}

// Reads stdin, returns utf8 []byte or error
func getStdIn() ([]byte, error) {
	fd, err := os.Stdin.Stat()
	if err != nil{
		return nil, err
	}
	if fd.Mode() & os.ModeNamedPipe == 0 {
		return nil, errors.New("stdin not a pipe")
	}else{
		stdInput, err := ioutil.ReadAll(os.Stdin)
		if err != nil{
			return nil, err
		}
		if len(stdInput) == 0 {
			return nil, errors.New("stdin is zero-length")
		}else{
			if ! utf8.Valid(stdInput){
				return nil, errors.New("stdin is not utf8")
			}else{
				return stdInput, nil
			}
		}
	}
}

// Writes file to disk or returns error 
func writeFile(s string, fn string) error {
	if _, err := os.Stat(fn); err != nil {
		if fd, err := os.Create(fn); err == nil {
			defer fd.Close()
			fd.WriteString(s)
		}else{
			fd.Close()
			return errors.New("Error creating file")
		}
	}else{
		return errors.New("Message already in queue")
	}
	return nil
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
	invokeLike := flag.String("l", "", "Like post_id from user@instance")
	invokeMessage := flag.String("m", "", "Message user@instance directly from stdin")
	invokeReply := flag.String("r", "", "Reply to post_id from user@instance from stdin")
	cfgFileName := flag.String("c", cfgFileNameDefault, "Configuration file")
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

	// For now our active account is always just default
	activeAccount = config["default"]

	// Determine why we are being invoked
	if *invokeUsage {
		usage()

	}else if *invokePost {
		dbg("Post")
		stdin, err := getStdIn()
		check(err)

		for _, v := range splitRunes(bytes.TrimRight(stdin, "\n"), maxTootRunes){
			msg := composePost(string(v))
			hash := sha256.Sum256([]byte(msg))
			err := writeFile(msg, strings.TrimRight(activeAccount["Outbox"], "/") + "/" + base64.RawURLEncoding.EncodeToString(hash[:]) + ".json")
			check(err)
		}

	}else if len(*invokeFollow) > 0 {
		dbg("Follow")

	}else if len(*invokeLike) > 0 {
		dbg("Like")

	}else if len(*invokeMessage) > 0 {
		dbg("Message")

		stdIn, err := getStdIn()
		check(err)
		var actorIDs []string
		for _, v := range strings.Split(*invokeMessage, ","){
			actorIDs = append(actorIDs, strings.TrimSpace(v))
		}
		actorIDs = expandActorIDs(actorIDs)

		if len(actorIDs) > 0 {
			for _, v := range splitRunes(bytes.TrimRight(stdIn, "\n"), maxTootRunes){
				msg := composeNote(string(v), actorIDs)
				hash := sha256.Sum256([]byte(msg))
				err := writeFile(msg, strings.TrimRight(activeAccount["Outbox"], "/") + "/" + base64.RawURLEncoding.EncodeToString(hash[:]) + ".json")
				check(err)
			}
		}else{
			die("Invalid Actor ID(s)")
		}

	}else if len(*invokeReply) > 0 {
		dbg("Reply")

	}else{ // Read toots from user's inbox
		dbg("Read")
	}

	dbg("Finished \n")
}
