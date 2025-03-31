# Wallets
# Hamid: 0xB7098929d914880eF9A18026F2290A9F23390D42



tidy:
	go mod tidy 
	go mod vendor 


generate-key:
	go run cmd/admin/main.go genkey -dir=zblock -account=hamid

scratch: 
	go run cmd/service/main.go