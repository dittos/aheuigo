package main

import (
    "fmt"
    "os"
    "io"
    "bufio"
    "strings"
    "unicode/utf8"
)

const (
    DirKeep = iota
    DirSet
    DirFlipX
    DirFlipY
    DirFlipXY
)

type Dir struct {
    dx, dy, flag int
}

type Cell struct {
    op, value int
    dir Dir
}

type Space struct {
    cells [][]Cell
    width, height int
}

const (
    OpKindPrint = 6
    OpKindInput = 7
)

const (
    OpDiv = 2
    OpAdd = 3
    OpMul = 4
    OpMod = 5
    OpDup = 8
    OpSwitch = 9
    OpMove = 10
    OpCmp = 12
    OpBranch = 14
    OpSub = 16
    OpSwap = 17
    OpExit = 18
)

const (
    OpPrintNum = 100 + iota
    OpPrintChar
    OpPop
    OpInputNum
    OpInputChar
    OpPush
)

var (
    dirs = map[int]Dir{
        0: Dir{1, 0, DirSet},
        2: Dir{2, 0, DirSet},
        4: Dir{-1, 0, DirSet},
        6: Dir{-2, 0, DirSet},
        8: Dir{0, -1, DirSet},
        12: Dir{0, -2, DirSet},
        13: Dir{0, 1, DirSet},
        17: Dir{0, 2, DirSet},
        18: Dir{flag: DirFlipY},
        19: Dir{flag: DirFlipXY},
        20: Dir{flag: DirFlipX},
    }
    values = []int{0, 2, 4, 4, 2, 5, 5, 3, 5, 7, 9, 9, 7, 9, 9, 8, 4, 4, 6, 2, 4, 1, 3, 4, 3, 4, 4, 3}
    requiredElems = map[int]int {
        OpDiv: 2,
        OpAdd: 2,
        OpMul: 2,
        OpMod: 2,
        OpDup: 1,
        OpMove: 1,
        OpCmp: 2,
        OpBranch: 1,
        OpSub: 2,
        OpSwap: 2,
        OpPrintNum: 1,
        OpPrintChar: 1,
        OpPop: 1,
    }
)

func IsHangul(ch rune) bool {
    return 0xAC00 <= ch && ch <= 0xD7A3
}

func Decode(r rune) (c Cell) {
    if IsHangul(r) {
        ch := int(r - 0xAC00)

        // Jungseong
        c.dir = dirs[ch / 28 % 21]

        // Jongseong
        c.value = ch % 28

        // Choseong
        op := ch / 28 / 21
        if op == OpKindPrint {
            switch c.value {
                case 21: op = OpPrintNum
                case 27: op = OpPrintChar
                default: op = OpPop
            }
        } else if op == OpKindInput {
            switch c.value {
                case 21: op = OpInputNum
                case 27: op = OpInputChar
                default: op, c.value = OpPush, values[c.value]
            }
        }
        c.op = op
    }
    return
}

func Input(r io.Reader) (space *Space) {
    reader := bufio.NewReader(r)
    space = &Space{nil, 0, 0}

    for {
        line, err := reader.ReadString('\n')
        if err == io.EOF {
            break
        }

        // Remove CR
        line = strings.Trim(line, "\r\n")

        // Expand space
        w := utf8.RuneCountInString(line)
        if w > space.width {
            space.width = w
        }
        space.height++

        cells := make([]Cell, 0, w)
        for _, ch := range line {
            cells = append(cells, Decode(ch))
        }
        space.cells = append(space.cells, cells)
    }

    return
}

type Context struct {
    curStorage int
    storage [28][]int
}

func (ctx *Context) SetStorage(s int) {
    ctx.curStorage = s
}

func (ctx *Context) StorageSize() int {
    return len(ctx.storage[ctx.curStorage])
}

func (ctx *Context) Push(value int) {
    ctx.PushTo(ctx.curStorage, value)
}

func (ctx *Context) PushTo(s int, value int) {
    ctx.storage[s] = append(ctx.storage[s], value)
}

func (ctx *Context) Pop() (value int) {
    s := ctx.curStorage
    if s == 21 {
        // Queue
        // XXX: does not free removed storage (GC?)
        value = ctx.storage[s][0]
        ctx.storage[s] = ctx.storage[s][1:]
    } else {
        // Stack
        last := len(ctx.storage[s]) - 1
        value = ctx.storage[s][last]
        ctx.storage[s] = ctx.storage[s][:last]
    }
    return
}

func (ctx *Context) Swap() {
    // TODO: more efficient impl.
    // XXX: queue?
    a := ctx.Pop()
    b := ctx.Pop()
    ctx.Push(a)
    ctx.Push(b)
}

func (ctx *Context) Dup() {
    // TODO: more efficient impl.
    s := ctx.curStorage
    if s == 21 {
        orig := ctx.storage[s]
        ctx.storage[s] = make([]int, len(orig) + 1)
        copy(ctx.storage[s][1:], orig)
        ctx.storage[s][0] = ctx.storage[s][1]
    } else {
        ctx.Push(ctx.storage[s][len(ctx.storage[s]) - 1])
    }
}

func (space *Space) Execute() {
    var x, y int
    var dir Dir

    ctx := new(Context)

    for {
        var cell Cell
        if x < len(space.cells[y]) {
            cell = space.cells[y][x]
        }

        // Update direction
        switch cell.dir.flag {
            case DirSet: dir = cell.dir
            case DirFlipX: dir.dx = -dir.dx
            case DirFlipY: dir.dy = -dir.dy
            case DirFlipXY: dir.dx, dir.dy = -dir.dx, -dir.dy
        }

        // Check for underflow
        if ctx.StorageSize() < requiredElems[cell.op] {
            dir.dx, dir.dy = -dir.dx, -dir.dy
            goto Next
        }

        switch cell.op {
            case OpDiv: a := ctx.Pop(); b := ctx.Pop(); ctx.Push(b / a)
            case OpAdd: a := ctx.Pop(); b := ctx.Pop(); ctx.Push(b + a)
            case OpMul: a := ctx.Pop(); b := ctx.Pop(); ctx.Push(b * a)
            case OpMod: a := ctx.Pop(); b := ctx.Pop(); ctx.Push(b % a)
            case OpPrintNum: fmt.Printf("%d", ctx.Pop())
            case OpPrintChar: fmt.Printf("%c", ctx.Pop())
            case OpPop: ctx.Pop()
            case OpInputNum: var a int; fmt.Scanf("%d", &a); ctx.Push(a)
            case OpInputChar: var a int; fmt.Scanf("%c", &a); ctx.Push(a)
            case OpPush: ctx.Push(cell.value)
            case OpDup: ctx.Dup()
            case OpSwitch: ctx.SetStorage(cell.value)
            case OpMove: ctx.PushTo(cell.value, ctx.Pop())
            case OpCmp: a := ctx.Pop(); b := ctx.Pop(); if b >= a { ctx.Push(1) } else { ctx.Push(0) }
            case OpBranch: if ctx.Pop() == 0 { dir.dx, dir.dy = -dir.dx, -dir.dy }
            case OpSub: a := ctx.Pop(); b := ctx.Pop(); ctx.Push(b - a)
            case OpSwap: ctx.Swap()
            case OpExit: return
        }

Next:
        x += dir.dx
        y += dir.dy

        // Wrap
        if y < 0 { y = space.height - 1 }
        if y >= space.height { y = 0 }
        if x < 0 { x = space.width - 1 }
        if x >= space.width { x = 0 }
    }
}

func main() {
    f, err := os.Open("test.aheui")
    if err != nil {
        return
    }
    defer f.Close()

    space := Input(f)
    space.Execute()
}
