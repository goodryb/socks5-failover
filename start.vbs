Set WshShell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
' 工作目录锁定为脚本所在目录，保证相对路径的数据库 socks5-failover.db 位置固定
WshShell.CurrentDirectory = fso.GetParentFolderName(WScript.ScriptFullName)
WshShell.Run "dist\socks5-failover.exe", 0, False
