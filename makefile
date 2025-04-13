# Wallets
# Hamid:  0xB7098929d914880eF9A18026F2290A9F23390D42
# John :  0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9
# Pavel:  0xdd6B972ffcc631a62CAE1BB9d80b7ff429c8ebA4
# Cesar:  0xbEE6ACE826eC3DE1B6349888B9151B92522F7F76
# Miner1: 0xFef311483Cc040e1A89fb9bb469eeB8A70935EF8
# Miner2: 0xb8Ee4c7ac4ca3269fEc242780D7D960bd6272a61


# curl 
# curl http://localhost:8000/genesis/list     
# curl http://localhost:8000/accounts/list
# curl http://localhost:8000/transactions/uncommit/list

tidy:
	go mod tidy 
	go mod vendor 


generate-key:
	go run cmd/admin/main.go genkey -dir=block -account=john

run: 
	CHAINGO_BENEFICIARY=miner1 go run cmd/service/main.go

run2:
	CHAINGO_BENEFICIARY=miner2 CHAINGO_PUBLIC_HOST=0.0.0.0:8080 CHAINGO_PRIVATE_HOST=0.0.0.0:9090 CHAINGO_STORAGE_DIR=block/miner2 go run cmd/service/main.go

# admin send -account=<name> -dir=<keys_dir> -url=<node> -nonce=<nonce> -from=<from_id> -to=<to_id> -value=<value> -tip=<tip> [-data=<hex_data>]
load:
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=1 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xbEE6ACE826eC3DE1B6349888B9151B92522F7F76 -value=100
	go run cmd/admin/main.go send -account=john -dir=block -nonce=1 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xbEE6ACE826eC3DE1B6349888B9151B92522F7F76 -value=75
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=2 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xdd6B972ffcc631a62CAE1BB9d80b7ff429c8ebA4 -value=150
	go run cmd/admin/main.go send -account=john -dir=block -nonce=2 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xdd6B972ffcc631a62CAE1BB9d80b7ff429c8ebA4 -value=200
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=3 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xbEE6ACE826eC3DE1B6349888B9151B92522F7F76 -value=60
	go run cmd/admin/main.go send -account=john -dir=block -nonce=3 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xdd6B972ffcc631a62CAE1BB9d80b7ff429c8ebA4 -value=300