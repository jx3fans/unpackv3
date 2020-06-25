package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	termbox "github.com/nsf/termbox-go"
	lzo "github.com/rasky/go-lzo"
	reg "golang.org/x/sys/windows/registry"
)

const limit bool = false

// Head 索引头信息
type Head struct {
	Sign    uint64      //签名
	Tmp     uint32      //版本
	Num     int32       //文件数量
	Trunk   [0x80]byte  //轨迹
	CfgName [0x170]byte //配置文件名称
}

// Item 索引
type Item struct {
	Hash    uint64 //文件名的hash
	Encpos  int64  //pakv3内的偏移
	Encsize int32  //在pakv3内的长度
}

func (item Item) String() string {
	return fmt.Sprintf("<Item(%016X %016X %08X)>\n", item.Hash, item.Encpos, item.Encsize)
}

// Items 索引列表
type Items []Item

func (p Items) Len() int {
	return len(p)
}

func (p Items) Swap(i int, j int) {
	p[i], p[j] = p[j], p[i]
}

// 降序 排列
func (p Items) Less(i int, j int) bool {
	return p[i].Hash < p[j].Hash
}

// Cfg 。。。
type Cfg struct {
	Sign    uint64     //签名
	Tmp     [0x10]byte //校验
	Version int32      //版本
	Num     int32      //pak数量
	Size    int64      //pak大小
}

// FileInfo 文件信息
type FileInfo struct {
	Tmp1    int64
	Tmp2    int32
	Srcsize int32 //原始大小
	Tmp4    int32
	Pos     int64 //文件偏移
	Encsize int32 //压缩后大小
	Tmp7    int32
	Enc     int32 //是否压缩过
}

// FileNameHash 将路径转换为hash。
//path := `scripts\achievement\ItemAcquire_Achievement.lua` //3A3064375F4C9628
//path := `scripts\Activity\6月16日运营活动\item\加成道具.lua` //37853E8D9C4E4A3E
func FileNameHash(fileName string) uint64 {
	hash := uint64(0)

	if fileName == "" {
		return hash
	}

	//预处理
	fileName = strings.ToLower(fileName)
	fileName = strings.Replace(fileName, `/`, `\`, -1)
	fileName = strings.Replace(fileName, `\\`, `\`, -1)
	if strings.HasPrefix(fileName, `\`) {
		fileName = fileName[1:]
	}

	if limit {
		data, _ := base64.StdEncoding.DecodeString(`ZGF0YQ==`)
		represent, _ := base64.StdEncoding.DecodeString(`cmVwcmVzZW50`)

		//对能解包的资源进行限制
		if !(strings.HasPrefix(fileName, string(data)) || strings.HasPrefix(fileName, string(represent))) {
			fmt.Println("暂只支持解包" + string(data) + "和" + string(represent) + "开头的文件")
			return hash
		}
	}

	//计算hash
	for _, v := range fileName {
		hash = uint64(v) + uint64(0x83)*hash
	}

	return hash
}

// ReadItems 读取全部索引
func ReadItems(jx3Root string) Items {
	// 打开文件
	fd, err := ioutil.ReadFile(jx3Root + "PakV3\\Trunk.DIR")

	if err != nil {
		log.Fatal(err)
		return nil
	}

	r := bytes.NewReader(fd)
	r.Seek(0, io.SeekStart)

	head := new(Head)
	if err := binary.Read(r, binary.LittleEndian, head); err != nil {
		log.Fatal(err)
		return nil
	}

	r.Seek(0x200, io.SeekStart)
	items := make([]Item, head.Num)
	if err := binary.Read(r, binary.LittleEndian, items); err != nil {
		log.Fatal(err)
		return nil
	}

	return items
}

// ReadCfg 读取配置文件
func ReadCfg(jx3Root string) *Cfg {
	// 打开文件
	fd, err := ioutil.ReadFile(jx3Root + "PakV3\\Package.CFG")

	if err != nil {
		log.Fatal(err)
		return nil
	}

	r := bytes.NewReader(fd)
	r.Seek(0, io.SeekStart)

	cfg := new(Cfg)
	if err := binary.Read(r, binary.LittleEndian, cfg); err != nil {
		log.Fatal(err)
		return nil
	}

	return cfg
}

// GetBytesFromPaks 从pak中获取文件内容
func GetBytesFromPaks(paks []*os.File, item Item) []byte {
	//计算文件所在的位置
	i := item.Encpos / 0x32000000
	p := item.Encpos % 0x32000000

	//读取内容块
	tmp := make([]byte, item.Encsize)
	paks[i].Seek(p, io.SeekStart)
	paks[i].Read(tmp)

	//读取文件索引信息
	r := bytes.NewReader(tmp)
	fi := new(FileInfo)
	if err := binary.Read(r, binary.LittleEndian, fi); err != nil {
		log.Fatal(err)
		return nil
	}

	//如果已经压缩，则解压后返回
	if fi.Enc == 1 {
		r.Seek(0x38, io.SeekStart)
		data, err := lzo.Decompress1X(r, int(fi.Encsize), int(fi.Srcsize))
		if err != nil {
			log.Fatal(err)
			return nil
		}
		return data
	}

	//如果未压缩则直接返回
	//if fi.Enc == 0 {
		return tmp[0x38:]
	//}
	
}

// SaveByteToFile 将[]byte保存为文件
func SaveByteToFile(fileName string, data []byte) {
	if fileName == "" {
		log.Fatal("保存的路径不能为空！")
		return
	}
	if data == nil {
		log.Fatal("要写入的文件内容为空！")
		return
	}

	fileName = strings.Replace(fileName, `\`, `/`, -1)

	os.MkdirAll(path.Dir(fileName), os.ModeDir)

	err := ioutil.WriteFile(fileName, data, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

// FormatPrintByte 格式化发方式打印byte
func FormatPrintByte(data []byte) {

	for i, v := range data {

		if i%8 == 0 {
			fmt.Print(" ")
		}

		if i%24 == 0 {
			fmt.Println()
		}

		fmt.Printf("%02X ", v)
	}
}

func init() {
	if err := termbox.Init(); err != nil {
		panic(err)
	}
	termbox.SetCursor(0, 0)
	termbox.HideCursor()
}

//任意键继续
func pause() {
	fmt.Println("请按任意键继续...")
Loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			break Loop
		}
	}
}

func printUsage() {
	fmt.Println("* 本工具只支持测试服客户端，本工具只支持64位操作系统。")
	fmt.Println("* 本工具会自动到注册表中读取客户端路径，并自动读取ls.txt列表解压文件")
	fmt.Println("* ls.txt文件必须为utf-8编码")
	if limit {
		fmt.Println("* 本工具已经限制只能解压data和represent开头的资源。")
	}
	fmt.Println("可选参数：")
	flag.PrintDefaults()
	pause()
}

func main() {

	jx3Root := flag.String("j", ``, `客户端根目录，如："E:\jx3_exp\"`)
	saveRoot := flag.String("s", `tmp\`, `解包后存放的位置，如："tmp\"`)
	listFile := flag.String("l", `ls.txt`, `列表文件，文件格式必须为UTF-8`)

	flag.Parse()

	//自动判断客户端的位置
	if *jx3Root == "" {
		k, err := reg.OpenKey(reg.LOCAL_MACHINE, `SOFTWARE\Wow6432Node\kingsoft\JX3\zhcn_exp`, reg.QUERY_VALUE)
		if err != nil {
			log.Fatal(err)
		}
		defer k.Close()

		s, _, err := k.GetStringValue("installPath")
		if err != nil {
			log.Fatal(err)
		}
		if s != "" && strings.Contains(s, `\bin\zhcn`) {
			i := strings.LastIndex(s, `\bin\zhcn`)
			*jx3Root = s[:i+1]
		} else {
			fmt.Println("未找到客户端，无法解压！")
			printUsage()
			return
		}
	}

	//判断列表文件是否可以读取
	_, err := os.Stat(*listFile)
	if err != nil && !os.IsExist(err) {
		fmt.Println("列表文件不存在，无法解压！")
		printUsage()
		return
	}

	//获取索引
	items := ReadItems(*jx3Root)
	sort.Sort(items)

	//获取pak列表
	cfg := ReadCfg(*jx3Root)
	paks := make([]*os.File, cfg.Num)
	for i := int32(0); i < cfg.Num; i++ {
		num := strconv.FormatInt(int64(i), 10)
		fd, err := os.Open(*jx3Root + "PakV3\\Package" + num + ".DAT")
		if err != nil {
			printUsage()
			log.Fatal(err)
			return
		}

		paks[i] = fd
	}

	//读取列表文件
	b, _ := ioutil.ReadFile(*listFile)
	s := string(b)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		//开始解压
		hash := FileNameHash(line)
		if hash == uint64(0) {
			continue
		}

		//二分查找
		i := sort.Search(len(items), func(i int) bool { return items[i].Hash >= hash })

		if i >= len(items) || items[i].Hash != hash {
			fmt.Println("不存在:" + line)
			continue
		}

		//解包
		data := GetBytesFromPaks(paks, items[i])
		SaveByteToFile(*saveRoot+line, data)

		//打印解包信息
		//fmt.Printf("%016X %s\n", hash, line)
		fmt.Println(line)
	}

}
