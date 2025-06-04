package main


func main(){
	go startServ()
	go startClient()
	startChromeWithProxy()
	select{}
}