package main


func main(){
	go startClient()
	startChromeWithProxy()
	select{}
}