package main

import "fmt"
import "bufio"
import "os"
import "flag"
import "strings"
import "io/ioutil"

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

// Read in config from passed filename
func readConfig(fName string){
	// Init global defaults
	config = make(map[string]string)
	config["Inbox"] = ""
	config["Outbox"] = ""

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

func main(){
	dbg("Starting \n")

	// Default values
	cfgFileNameDefault := "~/.tootc"
	
	cfgFileName := flag.String("cf", cfgFileNameDefault, "Configuration file")
	//readToots := flag.Bool("r", false, "Reading toots")
	flag.Parse()

	if *cfgFileName == cfgFileNameDefault {
		readConfig(os.Getenv("HOME") + strings.TrimLeft(cfgFileNameDefault, "~"))
	}else{
		readConfig(*cfgFileName)
	}

	fd, err := os.Stdin.Stat()
	check(err)

	if fd.Mode() & os.ModeNamedPipe == 0 {
		dbg("no pipe")
	}else{
		dbg("pipe")
		bytes, err := ioutil.ReadAll(os.Stdin)
		check(err)
		if len(bytes) > 0 {
			dbg(string(bytes))
		}
	}
	dbg("Finished \n")
}
