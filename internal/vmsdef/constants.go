package vmsdef

//go:generate go run ./gen -fabdef ../../reference/vms/fabdef.h -rabdef ../../reference/vms/rabdef.h -rmsdef ../../reference/vms/rmsdef.h -lnmdef ../../reference/vms/lnmdef.sdl -ssdef ../../reference/vms/ssdef.txt -devdef ../../reference/vms/devdef.sdl -jpidef ../../reference/vms/jpidef.sdl -iodef ../../reference/vms/iodef.sdl -statedef ../../reference/vms/statedef.txt -syidef ../../reference/vms/syidef.txt -dvidef ../../reference/vms/dvidef.txt -ttdef ../../reference/vms/ttdef.txt -sysmsg ../../reference/vms/sysmsg.txt -out constants_generated.go -msgout messages_generated.go
