package persistence

import(
	"bufio"
	"encoding/json"
	"os"
)

type WAL struct{
	file *os.File
}

func NewWAL(path string) (*WAL , error){
	file,err:=os.OpenFile(path,os.O_APPEND|os.O_CREATE|os.O_WRONLY,0644)
	if err!=nil{
		return nil,err
	}
	return &WAL{
		file: file,
	},nil
}

func (w *WAL) WriteCommand(cmd interface{})error{
	data,err:=json.Marshal(cmd)
	if err!=nil{
		return err
	}
	if _,err:=w.file.Write(append(data,'\n'));err!=nil{
		return err
	}
	return w.file.Sync()
}

func (w *WAL) Close() error{
	return w.file.Close()
}

func Replay(path string,applyFunc func(cmdBytes []byte) error) error{
	file,err:=os.Open(path)
	if err!=nil{
		if os.IsNotExist(err){
			return nil
		}
		return err
	}
	defer file.Close()

	scanner:=bufio.NewScanner(file)
	for scanner.Scan(){
		if err:=applyFunc(scanner.Bytes());err!=nil{
			return err
		}
	}
	return scanner.Err()
}