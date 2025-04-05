# Wallets
# Hamid: 0xB7098929d914880eF9A18026F2290A9F23390D42
# John : 0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9



# curl 
# curl http://localhost:8000/genesis/list     
# curl http://localhost:8000/accounts
# curl http://localhost:8000/transactions/uncommit/list

tidy:
	go mod tidy 
	go mod vendor 


generate-key:
	go run cmd/admin/main.go genkey -dir=block -account=john

run: 
	CHAINGO_BENEFICIARY=hamid go run cmd/service/main.go


# admin send -account=<name> -dir=<keys_dir> -url=<node> -nonce=<nonce> -from=<from_id> -to=<to_id> -value=<value> -tip=<tip> [-data=<hex_data>]
load:
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=1 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -value=100
	go run cmd/admin/main.go send -account=john -dir=block -nonce=1 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xB7098929d914880eF9A18026F2290A9F23390D42 -value=40
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=2 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -value=60
	go run cmd/admin/main.go send -account=john -dir=block -nonce=2 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xB7098929d914880eF9A18026F2290A9F23390D42 -value=200
	go run cmd/admin/main.go send -account=hamid -dir=block -nonce=3 -from=0xB7098929d914880eF9A18026F2290A9F23390D42 -to=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -value=610
	go run cmd/admin/main.go send -account=john -dir=block -nonce=3 -from=0xdF25785c81cf53f334fd05541Aa16bE8162CA8a9 -to=0xB7098929d914880eF9A18026F2290A9F23390D42 -value=400