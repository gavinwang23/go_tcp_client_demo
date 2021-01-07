package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net"
	"os"
)
//数据包类型
const (
	REPORT_PACKET = 0x01
)
//默认的服务器地址
var (
	server = "129.28.173.3:6969"
)
//数据包
type Packet struct {
	PacketType      byte
	PacketContent   []byte
}
//结构比特型报文报文头
type S_Head struct {
	PackHead		uint32	`json:"packHead"`	/*报文头  固定0x7E8118E7*/
	CurPackLen		uint32	`json:"curPackLen"`	/*当前包数据长度  包括“指令头”的数据长度*/
	TgtAddr			uint16	`json:"tgtAddr"`	/*目的地址*/
	SrcAddr			uint16	`json:"srcAddr"`	/*源地址*/
	DomainID		uint8	`json:"domainID"`	/*域ID  预留*/
	SubID			uint8	`json:"subID"`		/*主题ID  预留*/
	InfoType		uint16	`json:"infoType"`	/*信息类别号  各种交换的信息格式报分配唯一的编号*/
	InfoTime		int64	`json:"infoTime"`	/*发报日期时间  高32位是以秒值偏移量，以1970-1-1为基准；低32位是秒内计数值。单位：ns*/
	SeqNo			uint32	`json:"seqNo"`		/*序列号  同批数据的序列号相同，不同批数据的序列号不同；一批数据对应多个包。*/
	PackAmount		uint32	`json:"packAmount"`	/*包总数  当前发送的数据，总共分成几个包发送。默认1包*/
	PackNo			uint32	`json:"packNo"`		/*当前包号  当前发送的数据包序号。从1开始，当序列号不同时，当前包号清零，从1开始。*/
	DataLen			uint32	`json:"dataLen"`	/*数据总长度  所有“报文内容”长度之和，不包括“指令头”的数据总长度*/
	Version			uint16	`json:"version"`	/*版本号  高8位为为主版本号，低8位为副版本号，默认为1.00。副版本号最多为99。当有较大变更时，主版本号升级，副版本号重新归零。*/
	Reserve[6]		uint8	`json:"reserve"`	/*保留字段*/
}
// 默认S_Head
func defaultS_Head() S_Head {
	return S_Head {
		PackHead	:	0x7E8118E7,	/*报文头  固定0x7E8118E7*/
		CurPackLen	:	0,			/*当前包数据长度  包括“指令头”的数据长度*/
		TgtAddr		:	0,			/*目的地址*/
		SrcAddr		:	0,			/*源地址*/
		DomainID	:	0,			/*域ID  预留*/
		SubID		:	0,			/*主题ID  预留*/
		InfoType	:	0,			/*信息类别号  各种交换的信息格式报分配唯一的编号*/
		InfoTime	:	0,			/*发报日期时间  高32位是以秒值偏移量，以1970-1-1为基准；低32位是秒内计数值。单位：ns*/
		SeqNo		:	0,			/*序列号  同批数据的序列号相同，不同批数据的序列号不同；一批数据对应多个包。*/
		PackAmount	:	0,			/*包总数  当前发送的数据，总共分成几个包发送。默认1包*/
		PackNo		:	0,			/*当前包号  当前发送的数据包序号。从1开始，当序列号不同时，当前包号清零，从1开始。*/
		DataLen		:	0,			/*数据总长度  所有“报文内容”长度之和，不包括“指令头”的数据总长度*/
		Version		:	0,			/*版本号  高8位为为主版本号，低8位为副版本号，默认为1.00。副版本号最多为99。当有较大变更时，主版本号升级，副版本号重新归零。*/
		Reserve		:	[6]uint8{0,0,0,0,0,0},			/*保留字段*/
	}
}
//结构比特型报文报文尾
type S_Tail struct {
	CheckSum		uint32	`json:"checkSum"`	/*校验和  对报文头和报文内容进行累加和校验。暂时预留。*/
	PackTail		uint32	`json:"packTail"`	/*报文尾  固定0x8F9009F8*/
}
// 默认S_Tail
func defaultS_Tail() S_Tail {
	return S_Tail {
		CheckSum	:	0,
		PackTail	:	0x8F9009F8,
	}
}
//数据包
type ReportPacket struct {
	Content   		string`json:"content"`
	Rand         	int`json:"rand"`
	Timestamp   	int64`json:"timestamp"`
}

//客户端对象
type TcpClient struct {
	connection		*net.TCPConn
	hawkServer		*net.TCPAddr
	stopChan		chan struct{}
}

func main()  {
	//拿到服务器地址信息
	hawkServer,err := net.ResolveTCPAddr("tcp", server)
	if err != nil {
		fmt.Printf("hawk server [%s] resolve error: [%s]",server,err.Error())
		os.Exit(1)
	}
	//连接服务器
	connection,err := net.DialTCP("tcp",nil,hawkServer)
	if err != nil {
		fmt.Printf("connect to hawk server error: [%s]",err.Error())
		os.Exit(1)
	}
	client := &TcpClient{
		connection:connection,
		hawkServer:hawkServer,
		stopChan:make(chan struct{}),
	}
	//启动接收
	go client.receivePackets()

	//发送心跳的goroutine
	//go func() {
	//	heartBeatTick := time.Tick(2 * time.Second)
	//	for{
	//		select {
	//		case <-heartBeatTick:
	//			client.sendHeartPacket()
	//		case <-client.stopChan:
	//			return
	//		}
	//	}
	//}()

	//测试用的，开300个goroutine每秒发送一个包
	//for i:=0;i<300;i++ {
	//	go func() {
	//		sendTimer := time.After(1 * time.Second)
	//		for{
	//			select {
	//			case <-sendTimer:
	//				client.sendReportPacket()
	//				sendTimer = time.After(1 * time.Second)
	//			case <-client.stopChan:
	//				return
	//			}
	//		}
	//	}()
	//}
	//等待退出
	client.sendReportPacket()
	<-client.stopChan
}

// 接收数据包
func (client *TcpClient)receivePackets()  {
	reader := bufio.NewReader(client.connection)
	for {
		//承接上面说的服务器端的偷懒，我这里读也只是以\n为界限来读区分包
		msg, err := reader.ReadString('\n')
		if err != nil {
			//如果服务器关闭时的异常
			close(client.stopChan)
			break
		}
		fmt.Print(msg)
	}
}
//发送数据包
func (client *TcpClient)sendReportPacket()  {
	head := defaultS_Head()
	tail := defaultS_Tail()
	
	packetHead,err := json.Marshal(head)
	packetTail,err := json.Marshal(tail)
	fmt.Println(packetHead)
	if err!=nil{
		fmt.Println(err.Error())
	}
	
	//发送
	fmt.Printf("% X\n",packetHead)
	fmt.Println(string(packetHead))
	fmt.Println(packetHead)
	client.connection.Write(packetHead)
	client.connection.Write([]byte("这是body"))
	client.connection.Write(packetTail)
	fmt.Println("Send success!")
}

//使用的协议与服务器端保持一致
func EnPackSendData(sendBytes []byte) []byte {
	packetLength := len(sendBytes) + 8
	result := make([]byte,packetLength)
	result[0] = 0xFF
	result[1] = 0xFF
	result[2] = byte(uint16(len(sendBytes)) >> 8)
	result[3] = byte(uint16(len(sendBytes)) & 0xFF)
	copy(result[4:],sendBytes)
	sendCrc := crc32.ChecksumIEEE(sendBytes)
	result[packetLength-4] = byte(sendCrc >> 24)
	result[packetLength-3] = byte(sendCrc >> 16 & 0xFF)
	result[packetLength-2] = 0xFF
	result[packetLength-1] = 0xFE
	fmt.Println(result)
	return result
}

//拿一串随机字符
//func getRandString()string  {
//	length := rand.Intn(50)
//	strBytes := make([]byte,length)
//	for i:=0;i<length;i++ {
//		strBytes[i] = byte(rand.Intn(26) + 97)
//	}
//	return string(strBytes)
//}