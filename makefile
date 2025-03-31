tidy:
	go mod tidy 
	go mod vendor 


generate-key:
	go run cmd/admin/main.go genkey -dir=zblock -account=hamid