//go:build windows

package main

import (
    "bufio"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strconv"
    "strings"
    "syscall"
    "unicode/utf16"
    "unsafe"
)

const (
    errorSuccess     = 0
    errorNoMoreItems = 259
    errorMoreData    = 234

    keyRead       = 0x20019
    keyWrite      = 0x20006
    keyAllAccess  = 0xF003F

    regNone      = 0
    regSZ        = 1
    regExpandSZ  = 2
    regBinary    = 3
    regDWORD     = 4
    regMultiSZ   = 7
    regQWORD     = 11
)

var (
    advapi32          = syscall.NewLazyDLL("advapi32.dll")
    regOpenKeyExW      = advapi32.NewProc("RegOpenKeyExW")
    regCloseKey        = advapi32.NewProc("RegCloseKey")
    regEnumKeyExW      = advapi32.NewProc("RegEnumKeyExW")
    regEnumValueW      = advapi32.NewProc("RegEnumValueW")
    regQueryValueExW   = advapi32.NewProc("RegQueryValueExW")
    regSetValueExW     = advapi32.NewProc("RegSetValueExW")
    regCreateKeyExW    = advapi32.NewProc("RegCreateKeyExW")
    regDeleteValueW    = advapi32.NewProc("RegDeleteValueW")
    regDeleteTreeW     = advapi32.NewProc("RegDeleteTreeW")
    regRenameKey       = advapi32.NewProc("RegRenameKey")
)

type request struct {
    ID     any             `json:"id,omitempty"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params,omitempty"`
}

type response struct {
    ID     any         `json:"id,omitempty"`
    Result any         `json:"result,omitempty"`
    Error  *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type entry struct {
    Name      string `json:"name"`
    Path      string `json:"path"`
    Kind      string `json:"kind"`
    ValueType string `json:"valueType,omitempty"`
    Preview   string `json:"preview,omitempty"`
}

type listing struct {
    Path    string  `json:"path"`
    Entries []entry `json:"entries"`
}

type hive struct { name string; handle syscall.Handle }

var hives = []hive{
    {"HKEY_CLASSES_ROOT", syscall.Handle(0x80000000)},
    {"HKEY_CURRENT_USER", syscall.Handle(0x80000001)},
    {"HKEY_LOCAL_MACHINE", syscall.Handle(0x80000002)},
    {"HKEY_USERS", syscall.Handle(0x80000003)},
    {"HKEY_CURRENT_CONFIG", syscall.Handle(0x80000005)},
}

var aliases = map[string]string{
    "HKCR":"HKEY_CLASSES_ROOT", "HKCU":"HKEY_CURRENT_USER", "HKLM":"HKEY_LOCAL_MACHINE",
    "HKU":"HKEY_USERS", "HKCC":"HKEY_CURRENT_CONFIG",
}

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
    enc := json.NewEncoder(os.Stdout)
    for scanner.Scan() {
        var req request
        if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
            _ = enc.Encode(response{Error:&rpcError{Code:"invalid_request", Message:err.Error()}})
            continue
        }
        result, err := dispatch(req.Method, req.Params)
        if err != nil {
            _ = enc.Encode(response{ID:req.ID, Error:&rpcError{Code:"registry_error", Message:err.Error()}})
            continue
        }
        _ = enc.Encode(response{ID:req.ID, Result:result})
    }
}

func dispatch(method string, raw json.RawMessage) (any, error) {
    switch method {
    case "hello":
        return map[string]any{"protocol":1, "plugin":"reassembly.registry", "target":"windows-x64"}, nil
    case "registry.list":
        var p struct{ Path string `json:"path"` }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return list(p.Path)
    case "registry.read":
        var p struct{ Path string `json:"path"` }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return readValue(p.Path)
    case "registry.write":
        var p struct{ Path, Type, Data string }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return okResult(writeValue(p.Path,p.Type,p.Data))
    case "registry.createKey":
        var p struct{ Path string `json:"path"` }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return okResult(createKey(p.Path))
    case "registry.delete":
        var p struct{ Path string `json:"path"` }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return okResult(deletePath(p.Path))
    case "registry.rename":
        var p struct{ Path, NewName string }; if err := json.Unmarshal(raw,&p); err != nil { return nil,err }; return okResult(renamePath(p.Path,p.NewName))
    default:
        return nil, fmt.Errorf("unknown method %q", method)
    }
}

func okResult(err error) (any,error) { if err != nil { return nil,err }; return map[string]any{"ok":true},nil }

func normalize(path string) string {
    p := strings.TrimSpace(strings.ReplaceAll(path,"\\","/"))
    p = strings.TrimPrefix(strings.ToLower(p), "registry://")
    p = strings.Trim(p,"/")
    return p
}

func split(path string) (syscall.Handle,string,error) {
    p:=normalize(path); if p=="" { return 0,"",nil }
    parts:=strings.SplitN(p,"/",2); name:=strings.ToUpper(parts[0]); if c,ok:=aliases[name];ok{name=c}
    var root syscall.Handle
    found:=false
    for _,h:=range hives { if h.name==name { root=h.handle; found=true; break } }
    if !found { return 0,"",fmt.Errorf("unknown registry hive %q",parts[0]) }
    sub:=""; if len(parts)==2 { sub=strings.ReplaceAll(parts[1],"/","\\") }
    return root,sub,nil
}

func public(path string) string { p:=normalize(path); if p=="" {return "registry://"}; return "registry://"+strings.ReplaceAll(p,"\\","/") }
func join(base,name string) string { b:=strings.TrimSuffix(public(base),"/"); if b=="registry:" {b="registry://"}; if b=="registry://" {return b+name}; return b+"/"+name }

func utf16Ptr(s string) (*uint16,error) { return syscall.UTF16PtrFromString(s) }
func closeKey(h syscall.Handle) { if h!=0 { regCloseKey.Call(uintptr(h)) } }
func winerr(r uintptr) error { if r==errorSuccess{return nil}; return syscall.Errno(r) }

func open(root syscall.Handle, sub string, access uint32) (syscall.Handle,error) {
    if sub=="" { return root,nil }
    p,err:=utf16Ptr(sub); if err!=nil{return 0,err}; var out syscall.Handle
    r,_,_:=regOpenKeyExW.Call(uintptr(root),uintptr(unsafe.Pointer(p)),0,uintptr(access),uintptr(unsafe.Pointer(&out)))
    if r!=0{return 0,winerr(r)}; return out,nil
}

func list(path string) (listing,error) {
    root,sub,err:=split(path); if err!=nil{return listing{},err}
    if root==0 {
        out:=listing{Path:"registry://",Entries:make([]entry,0,len(hives))}
        for _,h:=range hives { out.Entries=append(out.Entries,entry{Name:h.name,Path:"registry://"+h.name,Kind:"key"}) }
        return out,nil
    }
    k,err:=open(root,sub,keyRead); if err!=nil{return listing{},err}; if sub!=""{defer closeKey(k)}
    out:=listing{Path:public(path),Entries:[]entry{}}
    for i:=uint32(0);;i++ {
        buf:=make([]uint16,16384); n:=uint32(len(buf));
        r,_,_:=regEnumKeyExW.Call(uintptr(k),uintptr(i),uintptr(unsafe.Pointer(&buf[0])),uintptr(unsafe.Pointer(&n)),0,0,0,0)
        if r==errorNoMoreItems{break}; if r!=0{return listing{},winerr(r)}
        name:=syscall.UTF16ToString(buf[:n]); out.Entries=append(out.Entries,entry{Name:name,Path:join(path,name),Kind:"key"})
    }
    for i:=uint32(0);;i++ {
        nameBuf:=make([]uint16,16384); nameLen:=uint32(len(nameBuf)); typ:=uint32(0); dataLen:=uint32(0)
        r,_,_:=regEnumValueW.Call(uintptr(k),uintptr(i),uintptr(unsafe.Pointer(&nameBuf[0])),uintptr(unsafe.Pointer(&nameLen)),0,uintptr(unsafe.Pointer(&typ)),0,uintptr(unsafe.Pointer(&dataLen)))
        if r==errorNoMoreItems{break}; if r!=0 && r!=errorMoreData{return listing{},winerr(r)}
        name:=syscall.UTF16ToString(nameBuf[:nameLen]); display:=name; if display==""{display="(default)"}
        preview:=""; if dataLen<=4096 { if v,e:=queryRaw(k,name);e==nil { preview=formatValue(v.typ,v.data) } }
        out.Entries=append(out.Entries,entry{Name:display,Path:join(path,display),Kind:"value",ValueType:typeName(typ),Preview:preview})
    }
    return out,nil
}

type rawValue struct { typ uint32; data []byte }
func queryRaw(k syscall.Handle,name string)(rawValue,error){
    actual:=name; if strings.EqualFold(name,"(default)"){actual=""}; p,err:=utf16Ptr(actual); if err!=nil{return rawValue{},err}
    typ:=uint32(0); n:=uint32(0); r,_,_:=regQueryValueExW.Call(uintptr(k),uintptr(unsafe.Pointer(p)),0,uintptr(unsafe.Pointer(&typ)),0,uintptr(unsafe.Pointer(&n)))
    if r!=0 && r!=errorMoreData{return rawValue{},winerr(r)}; data:=make([]byte,n); var ptr uintptr; if n>0{ptr=uintptr(unsafe.Pointer(&data[0]))}
    r,_,_ = regQueryValueExW.Call(uintptr(k),uintptr(unsafe.Pointer(p)),0,uintptr(unsafe.Pointer(&typ)),ptr,uintptr(unsafe.Pointer(&n)))
    if r!=0{return rawValue{},winerr(r)}; return rawValue{typ:typ,data:data[:n]},nil
}

func parentValue(path string)(syscall.Handle,string,string,error){
    root,sub,err:=split(path); if err!=nil{return 0,"","",err}; if root==0||sub==""{return 0,"","",errors.New("path is not a registry value")}
    i:=strings.LastIndex(sub,"\\"); parent:=""; name:=sub; if i>=0{parent=sub[:i];name=sub[i+1:]}; if strings.EqualFold(name,"(default)"){name=""}; return root,parent,name,nil
}

func readValue(path string)(any,error){
    root,parent,name,err:=parentValue(path); if err!=nil{return nil,err}; k,err:=open(root,parent,keyRead); if err!=nil{return nil,err}; if parent!=""{defer closeKey(k)}
    v,err:=queryRaw(k,name); if err!=nil{return nil,err}; return map[string]any{"path":public(path),"type":typeName(v.typ),"data":formatValue(v.typ,v.data)},nil
}

func typeName(t uint32)string{switch t{case regSZ:return"REG_SZ";case regExpandSZ:return"REG_EXPAND_SZ";case regBinary:return"REG_BINARY";case regDWORD:return"REG_DWORD";case regMultiSZ:return"REG_MULTI_SZ";case regQWORD:return"REG_QWORD";case regNone:return"REG_NONE";default:return fmt.Sprintf("REG_%d",t)}}

func formatValue(t uint32,b []byte)string{
    switch t {
    case regSZ,regExpandSZ,regMultiSZ:
        if len(b)%2!=0{return string(b)}; u:=make([]uint16,len(b)/2); for i:=range u{u[i]=binary.LittleEndian.Uint16(b[i*2:])}; s:=string(utf16.Decode(u)); s=strings.TrimRight(s,"\x00"); if t==regMultiSZ{return strings.ReplaceAll(s,"\x00","\n")}; return s
    case regDWORD: if len(b)>=4{return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(b)),10)}
    case regQWORD: if len(b)>=8{return strconv.FormatUint(binary.LittleEndian.Uint64(b),10)}
    case regBinary,regNone: return strings.ToUpper(hex.EncodeToString(b))
    }
    return strings.ToUpper(hex.EncodeToString(b))
}

func parseData(t string,s string)(uint32,[]byte,error){
    typ:=strings.ToUpper(strings.TrimSpace(t)); switch typ {
    case "REG_SZ","REG_EXPAND_SZ","REG_MULTI_SZ":
        rt:=uint32(regSZ); if typ=="REG_EXPAND_SZ"{rt=regExpandSZ}; if typ=="REG_MULTI_SZ"{rt=regMultiSZ;s=strings.ReplaceAll(s,"\r\n","\n");s=strings.ReplaceAll(s,"\n","\x00")}
        u:=utf16.Encode([]rune(s+"\x00")); if rt==regMultiSZ{u=append(u,0)}; b:=make([]byte,len(u)*2); for i,v:=range u{binary.LittleEndian.PutUint16(b[i*2:],v)}; return rt,b,nil
    case "REG_DWORD": n,err:=strconv.ParseUint(strings.TrimSpace(s),0,32); if err!=nil{return 0,nil,err}; b:=make([]byte,4);binary.LittleEndian.PutUint32(b,uint32(n));return regDWORD,b,nil
    case "REG_QWORD": n,err:=strconv.ParseUint(strings.TrimSpace(s),0,64); if err!=nil{return 0,nil,err}; b:=make([]byte,8);binary.LittleEndian.PutUint64(b,n);return regQWORD,b,nil
    case "REG_BINARY","REG_NONE": clean:=strings.NewReplacer(" ","",":","","-","").Replace(s); b,err:=hex.DecodeString(clean); if err!=nil{return 0,nil,err}; if typ=="REG_NONE"{return regNone,b,nil};return regBinary,b,nil
    default:return 0,nil,fmt.Errorf("unsupported registry type %q",t)
    }
}

func writeValue(path,t,data string)error{
    root,parent,name,err:=parentValue(path); if err!=nil{return err}; k,err:=open(root,parent,keyWrite); if err!=nil{return err}; if parent!=""{defer closeKey(k)}; typ,b,err:=parseData(t,data);if err!=nil{return err}; p,_:=utf16Ptr(name); var ptr uintptr;if len(b)>0{ptr=uintptr(unsafe.Pointer(&b[0]))}; r,_,_:=regSetValueExW.Call(uintptr(k),uintptr(unsafe.Pointer(p)),0,uintptr(typ),ptr,uintptr(len(b)));return winerr(r)
}

func createKey(path string)error{
    root,sub,err:=split(path);if err!=nil{return err};if root==0||sub==""{return errors.New("cannot create registry root/hive")};p,_:=utf16Ptr(sub);var out syscall.Handle;var disp uint32
    r,_,_:=regCreateKeyExW.Call(uintptr(root),uintptr(unsafe.Pointer(p)),0,0,0,uintptr(keyAllAccess),0,uintptr(unsafe.Pointer(&out)),uintptr(unsafe.Pointer(&disp)));if r==0{closeKey(out)};return winerr(r)
}

func deletePath(path string)error{
    root,sub,err:=split(path);if err!=nil{return err};if root==0||sub==""{return errors.New("cannot delete registry root/hive")}
    if k,e:=open(root,sub,keyRead);e==nil{closeKey(k);p,_:=utf16Ptr(sub);r,_,_:=regDeleteTreeW.Call(uintptr(root),uintptr(unsafe.Pointer(p)));return winerr(r)}
    root,parent,name,err:=parentValue(path);if err!=nil{return err};k,err:=open(root,parent,keyWrite);if err!=nil{return err};if parent!=""{defer closeKey(k)};p,_:=utf16Ptr(name);r,_,_:=regDeleteValueW.Call(uintptr(k),uintptr(unsafe.Pointer(p)));return winerr(r)
}

func renamePath(path,newName string)error{
    newName=strings.TrimSpace(newName);if newName==""||strings.ContainsAny(newName,"/\\"){return errors.New("newName must be one registry name")}
    root,sub,err:=split(path);if err!=nil{return err};if root==0||sub==""{return errors.New("cannot rename registry root/hive")}
    if k,e:=open(root,sub,keyRead);e==nil{closeKey(k);i:=strings.LastIndex(sub,"\\");parent:="";old:=sub;if i>=0{parent=sub[:i];old=sub[i+1:]};pk,e:=open(root,parent,keyAllAccess);if e!=nil{return e};if parent!=""{defer closeKey(pk)};op,_:=utf16Ptr(old);np,_:=utf16Ptr(newName);r,_,_:=regRenameKey.Call(uintptr(pk),uintptr(unsafe.Pointer(op)),uintptr(unsafe.Pointer(np)));return winerr(r)}
    root,parent,name,err:=parentValue(path);if err!=nil{return err};k,err:=open(root,parent,keyAllAccess);if err!=nil{return err};if parent!=""{defer closeKey(k)};v,err:=queryRaw(k,name);if err!=nil{return err};np,_:=utf16Ptr(newName);var ptr uintptr;if len(v.data)>0{ptr=uintptr(unsafe.Pointer(&v.data[0]))};r,_,_:=regSetValueExW.Call(uintptr(k),uintptr(unsafe.Pointer(np)),0,uintptr(v.typ),ptr,uintptr(len(v.data)));if r!=0{return winerr(r)};op,_:=utf16Ptr(name);r,_,_=regDeleteValueW.Call(uintptr(k),uintptr(unsafe.Pointer(op)));return winerr(r)
}
