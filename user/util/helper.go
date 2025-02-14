package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func StrToNum(text string) uint32 {
	value, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		panic(err)
	}
	return uint32(value)
}

func StrToNum64(text string) uint64 {
	value, err := strconv.ParseUint(text, 0, 64)
	if err != nil {
		panic(err)
	}
	return value
}

func IntToBytes(n int) []byte {
	x := uint32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, x)
	return bytesBuffer.Bytes()
}

func UIntToBytes(x uint32) []byte {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, x)
	return bytesBuffer.Bytes()
}

func FindRet(lib_path string, sym_offset, sym_size int64) (string, error) {
	file, err := os.Open(lib_path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	sym_data := make([]byte, sym_size)
	_, err = file.Seek(sym_offset, 0)
	if err != nil {
		return "", err
	}

	_, err = file.Read(sym_data)
	if err != nil {
		return "", err
	}

	// https://armconverter.com/?code=RET -> C0035FD6
	return FindIns(sym_offset, sym_data, []byte{0xC0, 0x03, 0x5F, 0xD6}), nil
}

func FindIns(sym_offset int64, sym_data, ins []byte) string {
	var result []string
	ret_offset := bytes.Index(sym_data, ins)
	for ret_offset > 0 {
		result = append(result, fmt.Sprintf("0x%x", sym_offset+int64(ret_offset)))
		sym_data = sym_data[ret_offset:]
		ret_offset = bytes.Index(sym_data, ins)
	}
	return strings.Join(result, ",")
}

type PackageInfo struct {
	Name string
	Uid  uint32
}

type PackageInfos struct {
	items []PackageInfo
}

func (this *PackageInfos) FindPackageByName(name string) (bool, PackageInfo) {
	for _, item := range this.items {
		if item.Name == name {
			return true, item
		}
	}
	return false, PackageInfo{}
}

func (this *PackageInfos) FindPackageByUid(uid uint32) (bool, PackageInfo) {
	for _, item := range this.items {
		if item.Uid == uid {
			return true, item
		}
	}
	return false, PackageInfo{}
}

func (this *PackageInfos) FindPackageByPid(uid uint32) (bool, PackageInfo) {
	for _, item := range this.items {
		if item.Uid == uid {
			return true, item
		}
	}
	return false, PackageInfo{}
}

func (this *PackageInfos) FindUidByPid(pid uint32) uint32 {
	// 安卓上检查进程架构的两种方法
	// 1. 检查 maps 中 linker 的名字 => linker or linker64
	// 2. 检查 maps 中 app_process 的名字 => app_process or app_process64

	// cat /proc/22812/maps | grep -m1 bin/linker
	// cat /proc/22812/maps | grep -m1 bin/app_process

	// user => u0_xxx
	// uid => 10xxx
	// uid=$(ps -o user= -p 22812) && id -u $uid
	// sh -c ps -o uid= -p 22812

	// pid_str := strconv.FormatUint(uint64(pid), 10)
	// lines, err := RunCommand("sh", "-c", fmt.Sprintf("uid=$(ps -o user= -p %s ) && id -u $uid", pid_str))
	lines, err := RunCommand("sh", "-c", fmt.Sprintf("ps -o uid= -p %d", pid))
	if err != nil {
		panic(err)
	}
	value, err := strconv.ParseUint(lines, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("find uid by pid failed, are you sure pid=%d exists ?", pid))
	}
	return uint32(value)
}

func Get_PackageInfos() *PackageInfos {
	// https://zhuanlan.zhihu.com/p/31124919
	// /data/system/packages.list
	content, err := ioutil.ReadFile("/data/system/packages.list")
	if err != nil {
		panic(err)
	}
	var pis PackageInfos
	lines := strings.TrimSpace(string(content))
	for _, line := range strings.Split(lines, "\n") {
		parts := strings.Split(line, " ")
		value, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			panic(err)
		}
		pis.items = append(pis.items, PackageInfo{parts[0], uint32(value)})
	}
	return &pis
}

func ReadMapsByPid(pid uint32) (string, error) {
	filename := fmt.Sprintf("/proc/%d/maps", pid)
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func ParseSignal(signal string) uint32 {
	if signal == "" {
		return 0
	}
	sigs := map[string]uint32{
		"SIGABRT":   uint32(syscall.SIGABRT),
		"SIGALRM":   uint32(syscall.SIGALRM),
		"SIGBUS":    uint32(syscall.SIGBUS),
		"SIGCHLD":   uint32(syscall.SIGCHLD),
		"SIGCLD":    uint32(syscall.SIGCLD),
		"SIGCONT":   uint32(syscall.SIGCONT),
		"SIGFPE":    uint32(syscall.SIGFPE),
		"SIGHUP":    uint32(syscall.SIGHUP),
		"SIGILL":    uint32(syscall.SIGILL),
		"SIGINT":    uint32(syscall.SIGINT),
		"SIGIO":     uint32(syscall.SIGIO),
		"SIGIOT":    uint32(syscall.SIGIOT),
		"SIGKILL":   uint32(syscall.SIGKILL),
		"SIGPIPE":   uint32(syscall.SIGPIPE),
		"SIGPOLL":   uint32(syscall.SIGPOLL),
		"SIGPROF":   uint32(syscall.SIGPROF),
		"SIGPWR":    uint32(syscall.SIGPWR),
		"SIGQUIT":   uint32(syscall.SIGQUIT),
		"SIGSEGV":   uint32(syscall.SIGSEGV),
		"SIGSTKFLT": uint32(syscall.SIGSTKFLT),
		"SIGSTOP":   uint32(syscall.SIGSTOP),
		"SIGSYS":    uint32(syscall.SIGSYS),
		"SIGTERM":   uint32(syscall.SIGTERM),
		"SIGTRAP":   uint32(syscall.SIGTRAP),
		"SIGTSTP":   uint32(syscall.SIGTSTP),
		"SIGTTIN":   uint32(syscall.SIGTTIN),
		"SIGTTOU":   uint32(syscall.SIGTTOU),
		"SIGUNUSED": uint32(syscall.SIGUNUSED),
		"SIGURG":    uint32(syscall.SIGURG),
		"SIGUSR1":   uint32(syscall.SIGUSR1),
		"SIGUSR2":   uint32(syscall.SIGUSR2),
		"SIGVTALRM": uint32(syscall.SIGVTALRM),
		"SIGWINCH":  uint32(syscall.SIGWINCH),
		"SIGXCPU":   uint32(syscall.SIGXCPU),
		"SIGXFSZ":   uint32(syscall.SIGXFSZ),
	}
	num, ok := sigs[signal]
	if ok {
		return num
	}
	panic(fmt.Sprintf("signal %s not support", signal))
}

func ParseReg(pid uint32, value uint64) (string, error) {
	info := "UNKNOWN"
	// 直接读取maps信息 计算value在什么地方 用于定位跳转目的地
	content, err := ReadMapsByPid(pid)
	if err != nil {
		return info, fmt.Errorf("Error when opening file:%v", err)
	}
	var (
		seg_start  uint64
		seg_end    uint64
		permission string
		seg_offset uint64
		device     string
		inode      uint64
		seg_path   string
	)
	for _, line := range strings.Split(content, "\n") {
		reader := strings.NewReader(line)
		n, err := fmt.Fscanf(reader, "%x-%x %s %x %s %d %s", &seg_start, &seg_end, &permission, &seg_offset, &device, &inode, &seg_path)
		if err == nil && n == 7 {
			if value >= seg_start && value < seg_end {
				offset := seg_offset + (value - seg_start)
				parts := strings.Split(seg_path, "/")
				seg_name := parts[len(parts)-1]
				info = fmt.Sprintf("%s + 0x%x", seg_name, offset)
				break
			}
		}
	}
	return info, err
}

func B2STrim(src []byte) string {
	return string(bytes.TrimSpace(bytes.Trim(src, "\x00")))
}

func B2S(bs []int8) string {
	ba := make([]byte, 0, len(bs))
	for _, b := range bs {
		ba = append(ba, byte(b))
	}
	return B2STrim(ba)
}

func I2B(bs []int8) []byte {
	ba := make([]byte, 0, len(bs))
	for _, b := range bs {
		ba = append(ba, byte(b))
	}
	return ba
}

const (
	GROUP_NONE   uint32 = 1 << 0
	GROUP_ROOT   uint32 = 1 << 1
	GROUP_SYSTEM uint32 = 1 << 2
	GROUP_SHELL  uint32 = 1 << 3
	GROUP_APP    uint32 = 1 << 4
	GROUP_ISO    uint32 = 1 << 5
)

const (
	SYS_WHITELIST_START uint32 = 0x400
	SYS_BLACKLIST_START uint32 = SYS_WHITELIST_START + 0x400
	UID_WHITELIST_START uint32 = SYS_BLACKLIST_START + 0x400
	UID_BLACKLIST_START uint32 = UID_WHITELIST_START + 0x400
	PID_WHITELIST_START uint32 = UID_BLACKLIST_START + 0x400
	PID_BLACKLIST_START uint32 = PID_WHITELIST_START + 0x400
	TID_WHITELIST_START uint32 = PID_BLACKLIST_START + 0x400
	TID_BLACKLIST_START uint32 = TID_WHITELIST_START + 0x400
)

var START_OFFSETS map[uint32]string = map[uint32]string{
	SYS_WHITELIST_START: "SYS_WHITELIST_START",
	SYS_BLACKLIST_START: "SYS_BLACKLIST_START",
	UID_WHITELIST_START: "UID_WHITELIST_START",
	UID_BLACKLIST_START: "UID_BLACKLIST_START",
	PID_WHITELIST_START: "PID_WHITELIST_START",
	PID_BLACKLIST_START: "PID_BLACKLIST_START",
	TID_WHITELIST_START: "TID_WHITELIST_START",
	TID_BLACKLIST_START: "TID_BLACKLIST_START",
}

// 格式化输出相关

const CHUNK_SIZE = 16
const CHUNK_SIZE_HALF = CHUNK_SIZE / 2

const (
	COLORRESET  = "\033[0m"
	COLORRED    = "\033[31m"
	COLORGREEN  = "\033[32m"
	COLORYELLOW = "\033[33m"
	COLORBLUE   = "\033[34m"
	COLORPURPLE = "\033[35m"
	COLORCYAN   = "\033[36m"
	COLORWHITE  = "\033[37m"
)

func dumpByteSlice(b []byte, perfix string) *bytes.Buffer {
	var a [CHUNK_SIZE]byte
	bb := new(bytes.Buffer)
	n := (len(b) + (CHUNK_SIZE - 1)) &^ (CHUNK_SIZE - 1)

	for i := 0; i < n; i++ {

		// 序号列
		if i%CHUNK_SIZE == 0 {
			bb.WriteString(perfix)
			bb.WriteString(fmt.Sprintf("%04d", i))
		}

		// 长度的一半，则输出4个空格
		if i%CHUNK_SIZE_HALF == 0 {
			bb.WriteString("    ")
		} else if i%(CHUNK_SIZE_HALF/2) == 0 {
			bb.WriteString("  ")
		}

		if i < len(b) {
			bb.WriteString(fmt.Sprintf(" %02X", b[i]))
		} else {
			bb.WriteString("  ")
		}

		// 非ASCII 改为 .
		if i >= len(b) {
			a[i%CHUNK_SIZE] = ' '
		} else if b[i] < 32 || b[i] > 126 {
			a[i%CHUNK_SIZE] = '.'
		} else {
			a[i%CHUNK_SIZE] = b[i]
		}

		// 如果到达size长度，则换行
		if i%CHUNK_SIZE == (CHUNK_SIZE - 1) {
			bb.WriteString(fmt.Sprintf("    %s\n", string(a[:])))
		}
	}
	return bb
}

func RestoreByteSlice(str string) []byte {
	text, err := strconv.Unquote("\"" + str + "\"")
	if err != nil {
		panic(fmt.Sprintf("RestoreByteSlice failed for %s", str))
	}
	return []byte(text)
}

func PrettyByteSlice(buffer []byte) string {
	var out strings.Builder
	for _, b := range buffer {
		if b >= 32 && b <= 126 {
			out.WriteByte(b)
		} else {
			out.WriteString(fmt.Sprintf("\\x%02x", b))
		}
	}
	return out.String()
}

func ArrayFormat(arr interface{}) string {
	var result []string
	switch arr := arr.(type) {
	case []int32:
		for _, num := range arr {
			result = append(result, fmt.Sprintf("%d", num))
		}
	case []uint32:
		for _, num := range arr {
			result = append(result, fmt.Sprintf("%d", num))
		}
	case []int64:
		for _, num := range arr {
			result = append(result, fmt.Sprintf("%d", num))
		}
	case []uint64:
		for _, num := range arr {
			result = append(result, fmt.Sprintf("%d", num))
		}
	default:
		return ""
	}
	return "[" + strings.Join(result, ", ") + "]"
}

func HexDump(buffer []byte, color string) string {
	b := dumpByteSlice(buffer, color)
	b.WriteString(COLORRESET)
	return b.String()
}

func HexDumpPure(buffer []byte) string {
	b := dumpByteSlice(buffer, "")
	return b.String()
}

func HexDumpGreen(buffer []byte) string {
	b := dumpByteSlice(buffer, COLORGREEN)
	b.WriteString(COLORRESET)
	return b.String()
}

const (
	HW_BREAKPOINT_EMPTY   uint32 = 0
	HW_BREAKPOINT_R       uint32 = 1
	HW_BREAKPOINT_W       uint32 = 2
	HW_BREAKPOINT_RW      uint32 = HW_BREAKPOINT_R | HW_BREAKPOINT_W
	HW_BREAKPOINT_X       uint32 = 4
	HW_BREAKPOINT_INVALID uint32 = HW_BREAKPOINT_RW | HW_BREAKPOINT_X
)

func RunCommand(executable string, args ...string) (string, error) {
	cmd := exec.Command(executable, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}
