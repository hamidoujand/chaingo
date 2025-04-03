# Wallets
# Hamid: 0xB7098929d914880eF9A18026F2290A9F23390D42
# John : 0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9



tidy:
	go mod tidy 
	go mod vendor 


generate-key:
	go run cmd/admin/main.go genkey -dir=block -account=john

run: 
	CHAINGO_BENEFICIARY=hamid go run cmd/service/main.go