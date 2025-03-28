package main
import (
    "fmt"
    "os"
    "time"

    "github.com/sony/sonyflake"
)
func getMachineID() (uint16, error) {
	var machineID uint16
	var err error
	machineID = readMachineIDFromLocalFile()
	if machineID == 0 {
		machineID, err = generateMachineID()
		if err != nil {
			return 0, err
		}
	}
	return machineID, nil
}

func checkMachineID(machineID uint16) bool {
	saddResult, err := saddMachineIDToRedisSet()
	if err != nil || saddResult == 0 {
		return true
	}
	err = saveMachineIDToLocalFile(machineID)
	if err != nil {
		return true
	}
	return false
}
//undefined: readMachineIDFromLocalFile
//undefined: generateMachineID
//undefined: saddMachineIDToRedisSet
//undefined: saveMachineIDToLocalFile
func main(){
	t,_:=time.Parse("2016-01-01","2019-01-11")
	settings:=sonyflake.Settings{
		StartTime:t,
		MachineID:getMachineID,
		CheckMachineID:checkMachineID,
	}
	sf:=sonyflake.NewSonyflake(settings)
	id,err:=sf.NextID()
	if err!=nil{
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(id)
}