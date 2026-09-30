' 静默停止后台运行的 socks5-failover（与 start.vbs 配对，不弹命令行窗口）
Set WshShell = CreateObject("WScript.Shell")

' 同时覆盖本地构建产物与 Release 下载产物的进程名
names = Array("socks5-failover.exe", "socks5-failover-windows-amd64.exe")

For Each n In names
    ' 用 /f 强制结束：后台无窗口进程收不到关闭窗口消息，只能按映像名终止。
    ' 流量增量每 30 秒落库并回收 WAL，故强杀最多丢失 30 秒统计，不会损坏数据库。
    WshShell.Run "taskkill /f /im " & n, 0, True
Next
