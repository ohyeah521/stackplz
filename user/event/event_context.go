package event

import (
    "bytes"
    "encoding/binary"
    "encoding/json"
    "errors"
    "fmt"
    "stackplz/user/common"
    "stackplz/user/config"
    "stackplz/user/util"
    "strconv"
    "strings"
    "syscall"

    "golang.org/x/sys/unix"
)

type LibArg struct {
    Abi       uint64
    Regs      [33]uint64
    StackSize uint64
    DynSize   uint64
}

type UnwindOption struct {
    RegMask uint64
    ShowPC  bool
}

type UnwindBuf struct {
    Abi       uint64
    Regs      [33]uint64
    StackSize uint64
    Data      []byte
    DynSize   uint64
}

func (this *UnwindBuf) GetLibArg() *LibArg {
    arg := &LibArg{}
    arg.Abi = this.Abi
    arg.Regs = this.Regs
    arg.StackSize = this.StackSize
    arg.DynSize = this.DynSize
    return arg
}

func (this *UnwindBuf) ParseContext(buf *bytes.Buffer) (err error) {
    if err = binary.Read(buf, binary.LittleEndian, &this.Abi); err != nil {
        return err
    }
    if err = binary.Read(buf, binary.LittleEndian, &this.Regs); err != nil {
        return err
    }
    if err = binary.Read(buf, binary.LittleEndian, &this.StackSize); err != nil {
        return err
    }

    stack_data := make([]byte, this.StackSize)
    if err = binary.Read(buf, binary.LittleEndian, &stack_data); err != nil {
        return err
    }
    this.Data = stack_data

    if err = binary.Read(buf, binary.LittleEndian, &this.DynSize); err != nil {
        return err
    }
    return nil
}

type RegsBuf struct {
    Abi  uint64
    Regs [33]uint64
}

func (this *RegsBuf) ParseContext(buf *bytes.Buffer) (err error) {
    if err = binary.Read(buf, binary.LittleEndian, &this.Abi); err != nil {
        return err
    }
    if err = binary.Read(buf, binary.LittleEndian, &this.Regs); err != nil {
        return err
    }
    return nil
}

type ContextEvent struct {
    CommonEvent
    config.ContextFields
    Stackinfo    string
    RegsBuffer   RegsBuf
    UnwindBuffer *UnwindBuf
}

func (this *ContextEvent) GetOffset(addr uint64) string {
    return maps_helper.GetOffset(this.Pid, addr)
}

func (this *ContextEvent) String() (s string) {
    s += fmt.Sprintf("event_id:%d ts:%d", this.EventId, this.Ts)
    s += fmt.Sprintf(", host_pid:%d, host_tid:%d", this.HostPid, this.HostTid)
    s += fmt.Sprintf(", Uid:%d, pid:%d, tid:%d", this.Uid, this.Pid, this.Tid)
    s += fmt.Sprintf(", Comm:%s, argnum:%d", util.B2STrim(this.Comm[:]), this.Argnum)
    return s
}

func (this *ContextEvent) GetUUID() string {
    return fmt.Sprintf("%d_%d", this.Pid, this.Tid)
}

func (this *ContextEvent) GetEventId() uint32 {
    return this.EventId
}

func (this *ContextEvent) ParsePadding() (err error) {
    // 好在 SampleSize 是明确的 这样我们可以正确计算下一部分 perf 数据起始位置
    // ebpf库改为全部读取之后 这里的 4 是 PERF_SAMPLE_RAW 的 size
    padding_size := this.rec.SampleSize + 4 - uint32(this.buf.Cap()-this.buf.Len())
    if padding_size > 0 {
        payload := make([]byte, padding_size)
        if err = binary.Read(this.buf, binary.LittleEndian, &payload); err != nil {
            this.logger.Printf("ContextEvent EventId:%d RawSample:\n%s", this.EventId, util.HexDump(this.rec.RawSample, util.COLORRED))
            panic(err)
        }
    }
    return nil
}

func (this *ContextEvent) ParseEvent() (IEventStruct, error) {
    switch this.rec.RecordType {
    case unix.PERF_RECORD_SAMPLE:
        // 先把需要的基础信息解析出来
        err := this.ParseContext()
        if err != nil {
            panic(fmt.Sprintf("ContextEvent.ParseContext() err:%v", err))
        }

        EventId := this.GetEventId()
        switch EventId {
        case SYSCALL_ENTER, SYSCALL_EXIT:
            return nil, nil
        case UPROBE_ENTER:
            return nil, nil
        default:
            this.logger.Printf("ContextEvent.ParseEvent() unsupported EventId:%d\n", EventId)
            this.logger.Printf("ContextEvent.ParseEvent() PERF_RECORD_SAMPLE RawSample:\n" + util.HexDump(this.rec.RawSample, util.COLORRED))
            return nil, errors.New(fmt.Sprintf("PERF_RECORD_SAMPLE EventId is %d", EventId))
        }
    default:
        return this.CommonEvent.ParseEvent()
    }

}

func (this *ContextEvent) ParseContext() (err error) {
    if this.mconf.BrkKernel {
        return nil
    }
    this.buf = bytes.NewBuffer(this.rec.RawSample)
    if err = binary.Read(this.buf, binary.LittleEndian, &this.rec.SampleSize); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Ts); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.EventId); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.HostTid); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.HostPid); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Tid); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Pid); err != nil {
        return err
    }
    if this.mconf.KillSignal == uint32(syscall.SIGSTOP) && this.Pid != 0 {
        AddStopped(this.Pid)
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Uid); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Comm); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Argnum); err != nil {
        return err
    }
    if err = binary.Read(this.buf, binary.LittleEndian, &this.Padding); err != nil {
        return err
    }
    // 这一类的说明都是要关注的
    maps_helper.UpdatePidList(this.Pid)
    return nil
}

func (this *ContextEvent) Clone() IEventStruct {
    event := new(ContextEvent)
    return event
}

func (this *ContextEvent) GetStackTrace(s string) string {
    if this.mconf.RegName != "" {
        // 如果设置了寄存器名字 那么尝试从获取到的寄存器数据中取值计算偏移
        // 当然前提是取了寄存器数据
        var tmp_regs [33]uint64
        if this.rec.ExtraOptions.UnwindStack {
            tmp_regs = this.UnwindBuffer.Regs
        } else {
            tmp_regs = this.RegsBuffer.Regs
        }
        has_reg_value := false
        var regvalue uint64
        if strings.HasPrefix(this.mconf.RegName, "x") {
            parts := strings.SplitN(this.mconf.RegName, "x", 2)
            regno, _ := strconv.ParseUint(parts[1], 10, 32)
            if regno >= 0 && regno <= uint64(common.REG_ARM64_X29) {
                // 取到对应的寄存器值
                regvalue = tmp_regs[regno]
                has_reg_value = true
            }
        } else if this.mconf.RegName == "lr" {
            regvalue = tmp_regs[common.REG_ARM64_LR]
            has_reg_value = true
        }
        if has_reg_value {
            // maps_helper 的结构复杂 并且存在锁限制 不如直接读maps来的快
            // info := maps_helper.GetOffset(this.Pid, regvalue)
            // s += fmt.Sprintf(", Reg %s(%s)", this.mconf.RegName, info)
            info, err := util.ParseReg(this.Pid, regvalue)
            if err != nil {
                fmt.Printf("ParseReg for %s=0x%x failed", this.mconf.RegName, regvalue)
            } else {
                s += fmt.Sprintf(", Reg %s(%s)", this.mconf.RegName, info)
            }
        }
    } else if this.rec.ExtraOptions.ShowRegs {
        var tmp_regs [33]uint64
        if this.rec.ExtraOptions.UnwindStack {
            tmp_regs = this.UnwindBuffer.Regs
        } else {
            tmp_regs = this.RegsBuffer.Regs
        }
        regs := make(map[string]string)
        for regno := 0; regno <= int(common.REG_ARM64_X29); regno++ {
            regs[fmt.Sprintf("x%d", regno)] = fmt.Sprintf("0x%x", tmp_regs[regno])
        }
        regs["lr"] = fmt.Sprintf("0x%x", tmp_regs[common.REG_ARM64_LR])
        regs["sp"] = fmt.Sprintf("0x%x", tmp_regs[common.REG_ARM64_SP])
        regs["pc"] = fmt.Sprintf("0x%x", tmp_regs[common.REG_ARM64_PC])
        regs_info, err := json.Marshal(regs)
        if err != nil {
            regs_info = make([]byte, 0)
        }
        s += ", Regs:\n" + string(regs_info)
    }
    if this.Stackinfo != "" {
        if this.rec.ExtraOptions.ShowRegs {
            s += fmt.Sprintf("\nBacktrace:\n%s", this.Stackinfo)
        } else {
            s += fmt.Sprintf(", Backtrace:\n%s", this.Stackinfo)
        }
    }
    return s
}

func (this *ContextEvent) ParseContextStack() (err error) {
    this.Stackinfo = ""
    if this.rec.ExtraOptions.UnwindStack {
        // 读取完整的栈数据和寄存器数据 并解析为 UnwindBuf 结构体
        this.UnwindBuffer = &UnwindBuf{}
        err = this.UnwindBuffer.ParseContext(this.buf)
        if err != nil {
            panic(fmt.Sprintf("UnwindStack ParseContext failed, err:%v", err))
        }
        // 立刻获取堆栈信息 对于某些hook点前后可能导致maps发生变化的 堆栈可能不准确
        // 这里后续可以调整为只dlopen一次 拿到要调用函数的handle 不要重复dlopen
        content, err := util.ReadMapsByPid(this.Pid)
        if err != nil || this.mconf.ManualStack {
            // 直接读取 maps 失败 那么从 mmap2 事件中获取
            // 根据测试结果 有这样的情况 -> 即 fork 产生的子进程 那么应该查找其父进程 mmap2 事件
            maps_helper.SetLogger(this.logger)
            info, err := maps_helper.GetStack(this.Pid, this.UnwindBuffer)
            if err != nil {
                // this.logger.Printf("Error when opening file:%v", err)
                this.logger.Printf("Error when GetStack:%v", err)
            } else {
                this.Stackinfo = info
            }
            return nil
        }
        opt := &UnwindOption{}
        opt.RegMask = (1 << 33) - 1
        opt.ShowPC = this.mconf.ShowPC
        this.Stackinfo = ParseStack(content, opt, this.UnwindBuffer)
    } else if this.rec.ExtraOptions.ShowRegs {
        err = this.RegsBuffer.ParseContext(this.buf)
        if err != nil {
            panic(fmt.Sprintf("UnwindStack ParseContext failed, err:%v", err))
        }
    }
    return nil
}
