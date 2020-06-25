set GOPATH="D:\\golang\\gopath"

del unpackv3.exe
go build -ldflags "-s -w"

rem upx.exe -9 unpackv3.exe

rem call unpackv3.exe -j E:\jx3_exp\ 

pause