package main

import "fmt"

// Simple check func 
func check(e error){
	if e != nil {
		panic(e)
	}
}

// Simple debug func
func dbg(d bool, str string){
	if d{
		fmt.Println(str)		
	}
}


func main(){
	dbg(true, "Starting \n")
	
	dbg(true, "Finished \n")
}
