package ipfs

import(
	"bytes"
	"encoding/json"
	_"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func executeCommand(c string, logger *zap.Logger) ([]byte, error){
	cmd := exec.Command("bash", "-c", c)
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil{
		logger.Warn("failed to execute bash command", zap.Error(err), zap.String("error stdout", out.String()))
		return nil, err
	}

	return out.Bytes(), nil
}


func Dei(datatype uint32, cid, pass string, deindex bool, logger *zap.Logger){
	logger.Info("retrieving data from contract..", zap.String("cid", cid))

	datatypeStr := strconv.Itoa(int(datatype))
	var cmd string
	if datatype == 1 || datatype == 2{
		cmd = "dei " + datatypeStr + " " + cid + " " + pass + " " + strconv.FormatBool(deindex)
	} else{
		cmd = "dei " + datatypeStr + " " + cid + " " + strconv.FormatBool(deindex)
	}
	beg := time.Now()
	out, err := executeCommand(cmd, logger)
	logger.Info("dei time", zap.String("time", (time.Since(beg)).String()))
	if err != nil{
		logger.Info("retrieving data from contract: FAILED")
		logger.Debug(string(out))
	}
	logger.Info("retrieving data from contract: DONE")
}

func Subdei(datatype uint32, cid, pass string, deindex bool, logger *zap.Logger){
	logger.Info("retrieving data", zap.String("cid", cid))

	datatypeStr := strconv.Itoa(int(datatype))
	var cmd string
	if datatype == 1 || datatype == 2{
		cmd = "subdei " + datatypeStr + " " + cid + " " + pass
	} else{
		cmd = "subdei " + datatypeStr + " " + cid
	}
	out, err := executeCommand(cmd, logger)
	if err != nil{
		logger.Error("retrieving data: FAILED\n", zap.Error(err))
		logger.Debug(string(out))
	}
}

func ipfser(datatype uint32, filepath, pass string, logger *zap.Logger) (string, uint32, error){
	datatypeStr := strconv.Itoa(int(datatype))

	var cmd string
	if datatype == 1 || datatype == 2{
		cmd = "ipfser " + datatypeStr + " " + filepath + " " + pass
	} else{
		cmd = "ipfser " + datatypeStr + " " + filepath
	}
	logger.Info("executing ipfser..")
	beg := time.Now()
	out, err := executeCommand(cmd, logger)
	if err != nil{
		logger.Error("failed to execute ipfser command", zap.Error(err))
		return "", 0, err
	}
	logger.Info("successfuly added content to ipfs")
	logger.Info("ipfser time", zap.String("time", (time.Since(beg)).String()))
	outs := strings.Split(string(out), " ")
	if len(outs) != 2{
		logger.Error("unrecognized ipfser output")
		return "", 0, errors.New("unrecognized ipfser output")
	}

	cid, sizeStr := outs[0], outs[1]
	sizeInt, err := strconv.Atoi(sizeStr)
	if err != nil{
		logger.Error("failed to convert ipfs content size", zap.Error(err))
		return "", 0, err
	}
	size := uint32(1 + sizeInt /(1000000) ) //to get MB + some buffer
	logger.Info("content size", zap.Int("size", int(size)))

	return cid, size, nil
}

func storeToMFS(filepath string, logger *zap.Logger) (string, uint32, error){
	//add to ipfs
	logger.Info("adding file to ipfs")
	cmd := "curl -X POST -F file=@" + filepath + " http://127.0.0.1:5001/api/v0/add?" +
					"quiet=true&quieter=true&wrap-with-directory=true&pin=true"

	out, err := executeCommand(cmd, logger)
	if err != nil{
		return "", 0, errors.New("failed to add file to ipfs")
	}
	logger.Info("successfuly added file to ipfs")


	//convert to map
	var ipfs_out map[string]interface{}
	json.Unmarshal([]byte(out), &ipfs_out)

	hash := ipfs_out["Hash"].(string)
	sizeStr := ipfs_out["Size"].(string)
	sizeInt, err := strconv.Atoi(sizeStr)
	if err != nil{
		return "", 0, errors.New("failed to get ipfs file size")
	}
	size := uint32(1 + sizeInt /(1000000) ) //to get MB + some buffer


	//add to mfs
	logger.Info("adding file to mutable file system")
	cmd = "curl -X POST \"http://127.0.0.1:5001/api/v0/files/cp?arg=/ipfs/" + hash +
				"&arg=/\""
	out, err = executeCommand(cmd, logger)
	if err != nil{
		return "", 0, errors.New("failed to add file to mfs")
	}
	logger.Info("successfuly added file to mfs")

	return hash, size, nil
}


func startIpfsDaemon(logger *zap.Logger) error{
	_, err := exec.Command("bash", "-c", "pidof ipfs").CombinedOutput()
	if err == nil {
		logger.Info("ipfs daemon is ready")
		return nil
	}

	cmd := exec.Command("bash", "-c", "nohup ipfs daemon")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Start()
	for i:=0; i<30; i++{
		time.Sleep(1*time.Second)

		if err != nil{
			logger.Warn("failed to start ipfs daemon", zap.Error(err))
			return err
		}

		rows := strings.Split(out.String(), "\n")
		if len(rows) > 2{
			lastRow := rows[len(rows)-2]
			if lastRow=="Daemon is ready"{
				logger.Info("ipfs daemon is ready")
				return nil
			}
		}
	}

	logger.Warn("failed to start ipfs daemon", zap.Error(err))
	return errors.New("failed to start ipfs daemon with unknown reasons")
}


func Ipfser(datatype uint32, filepath, pass string, logger *zap.Logger) (string, uint32, error){
	if logger == nil{
		logger = zap.NewNop()
	}

	if err := startIpfsDaemon(logger); err != nil{
		return "", 0, errors.New("failed to start ipfs daemon")
	}

	/*
	cid, size, err := storeToMFS(filepath, logger)
	if err != nil{
		return "", 0, errors.New("failed to store to mutable file system")
	}
	*/

	cid, size, err := ipfser(datatype, filepath, pass, logger)
	if err != nil{
		return "", 0, errors.New("failed to store to ipfs")
	}

	return cid, size, nil
}

func ipfsPinAll(cid string, logger *zap.Logger) error{
	logger.Info("pinning index file..")
	if err := ipfsPin(cid, logger); err != nil{
		logger.Info("pinning index file: FAILED")
		return err
	}
	logger.Info("pinning index file: DONE")

	logger.Info("pinning content from index file..")
	cmd := "ipfs cat " + cid + " | while read line; do ipfs pin add $line; done"
	if _, err := executeCommand(cmd, logger); err != nil{
		logger.Info("pinning content from index file: FAILED")
		return err
	}
	logger.Info("pinning content from index file: DONE")
	return nil
}

func ipfsPin(cid string, logger *zap.Logger) error{
	//cmd := "curl -X POST \"http://127.0.0.1:5001/api/v0/pin/add?arg=/ipfs/" + cid + "\""
	cmd := "ipfs pin add " + cid
	if _, err := executeCommand(cmd, logger); err != nil{
		logger.Info("pinning cid: FAILED")
		return err
	}
	return nil
}

func IpfsUnpinAll(cid string, logger *zap.Logger){
	logger.Info("unpinning content from index file..")
	cmd := "ipfs cat " + cid + " | while read line; do ipfs pin rm $line; done"
	logger.Info("alo brate")
	if _, err := executeCommand(cmd, logger); err != nil{
		logger.Info("unpinning content from index file: FAILED")
	}
	logger.Info("unpinning content from index file: DONE")

	logger.Info("unpinning index file..")
	if err := ipfsUnpin(cid, logger); err != nil{
		logger.Info("unpinning index file: FAILED")
	}
	logger.Info("unpinning index file: DONE")
}

func ipfsUnpin(cid string, logger *zap.Logger) error{
	//cmd := "curl -X POST \"http://127.0.0.1:5001/api/v0/pin/rm?arg=" + cid + "\""
	cmd := "ipfs pin rm " + cid
	if _, err := executeCommand(cmd, logger); err != nil{
		logger.Warn("unpinning content: FAILED", zap.Error(err))
		return err
	}
	return nil
}

func HostContent(cid string, logger *zap.Logger) error{
	if logger == nil{
		logger = zap.NewNop()
	}

	if err := startIpfsDaemon(logger); err != nil{
		logger.Warn("failed to start ipfs daemon")
		return err
	}
	if err := ipfsPinAll(cid, logger); err != nil{
		logger.Warn("failed to pin index file + content to ipfs")
		return err
	}

	return nil
}
