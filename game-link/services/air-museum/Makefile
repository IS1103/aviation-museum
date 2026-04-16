# 產生 air_museum.proto 的 Go 程式碼到 proto/pb/
.PHONY: proto
proto:
	protoc --proto_path=proto --go_out=. --go_opt=module=air-museum proto/air_museum.proto
