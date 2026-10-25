/**
 * terminal-smart.js —— 终端智能输入
 * 1. 命令联想（参考腾讯 CloudShell：ghost 行内补全 → 接受/循环）
 * 2. 自然语言→命令（参考阿里云：意图解释 + 执行/修改/拒绝确认卡）
 * 纯前端离线引擎为主，可选用 AI 兜底；演示版（demo://）同样可用。
 */

// ========== 视觉宽度（终端单元格：CJK/全角=2，其余=1） ==========
const RE_CJK = /[\u1100-\u115F\u2E80-\u303E\u3041-\u33FF\u3400-\u4DBF\u4E00-\u9FFF\uA000-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE30-\uFE4F\uFF00-\uFF60\uFFE0-\uFFE6]/

export function visualWidth(s) {
  let n = 0
  for (const ch of String(s)) n += RE_CJK.test(ch) ? 2 : 1
  return n
}

// 按视觉宽度截断/填充；pad 字符默认空格
function fitVisual(s, width, align = 'left') {
  s = String(s)
  let t = '', w = 0
  for (const ch of s) {
    const cw = RE_CJK.test(ch) ? 2 : 1
    if (w + cw > width) break
    t += ch; w += cw
  }
  const pad = ' '.repeat(Math.max(0, width - w))
  return align === 'right' ? pad + t : t + pad
}

// ========== 1. 命令目录（高频运维命令全集） ==========
const COMMAND_CATALOG = [
  // 文件与目录
  'ls -la', 'ls -lh', 'ls -ltr', 'ls -alh', 'll', 'ls -F', 'ls -i',
  'pwd', 'whoami', 'id', 'groups', 'cd /etc', 'cd /var/log', 'cd ~', 'cd -',
  'tree -L 2', 'stat ', 'file ',
  // 系统状态
  'top', 'htop', 'uptime', 'w', 'who', 'last -20', 'last reboot',
  'ps aux', 'ps -ef', 'ps aux --sort=-%cpu | head -20', 'ps aux --sort=-%mem | head -20',
  'free -h', 'free -m', 'cat /proc/meminfo', 'vmstat 1 5',
  'df -h', 'df -i', 'df -hT', 'du -sh *', 'du -sh /var/log', 'du -sh /* | sort -rh | head',
  'iostat -xz 1 5',
  // 系统信息
  'uname -a', 'hostname', 'hostnamectl', 'cat /etc/os-release', 'cat /etc/redhat-release',
  'lscpu', 'lspci', 'lsblk', 'lsmod', 'env', 'printenv', 'date', 'timedatectl',
  // 网络
  'ip addr', 'ip route', 'ip -s link', 'ip neigh',
  'ss -tlnp', 'ss -antp', 'ss -ulnp', 'netstat -tlnp', 'netstat -i',
  'ping -c 4 ', 'traceroute ', 'dig +short ', 'nslookup ',
  'curl -I ', 'curl -s ', 'curl -v ', 'wget ',
  'lsof -i :80', 'lsof -nP | grep LISTEN', 'lsof -i -P -n',
  // 服务与日志
  'systemctl status nginx', 'systemctl restart nginx', 'systemctl reload nginx',
  'systemctl status mysql', 'systemctl status redis', 'systemctl status docker',
  'systemctl list-units --type=service --state=running', 'systemctl enable nginx', 'systemctl disable nginx',
  'journalctl -u nginx -n 100', 'journalctl -u nginx -f', 'journalctl --since "1 hour ago"',
  'tail -f /var/log/nginx/access.log', 'tail -f /var/log/nginx/error.log', 'tail -n 100 /var/log/messages',
  'tail -n 100 /var/log/syslog',
  // 配置查看
  'cat /etc/hosts', 'cat /etc/resolv.conf', 'cat /etc/fstab', 'cat /etc/passwd',
  'crontab -l', 'iptables -L -n -v', 'ufw status',
  // 容器
  'docker ps', 'docker ps -a', 'docker images', 'docker stats', 'docker logs -f ',
  'docker exec -it ', 'docker compose ps', 'docker compose logs -f',
  // 查找/过滤
  'grep -rn "" /etc --include="*.conf"', 'find / -name "*.conf" 2>/dev/null',
  'find . -mtime -1 -type f', 'grep -Rni "" /var/log --include="*.log" | head -50',
  // 权限/打包
  'chmod 755 ', 'chmod +x ', 'chown -R ',
  'tar -czvf backup.tar.gz ', 'tar -xzvf ', 'unzip -o ',
  // 进程控制
  'kill -9 ', 'kill -HUP ', 'pkill -f ',
  // 历史
  'history', 'history | grep ',
  // —— 来自 dbask.cn 全量命令（基础形式，带说明）——
  '7z', // 拥有极高压缩比的开源压缩软件
  'ab', // Apache服务器的性能测试工具
  'accept', // 指示打印系统接受发往指定目标打印机的打印任务
  'ack', // 比grep好用的文本搜索工具
  'ag', // ack 的升级版，C语言编写，更快更人性化
  'alias', // 定义或显示别名
  'apachectl', // Apache服务器前端控制工具
  'apk', // Alpine Linux 下的包管理工具
  'apropos', // 在 whatis 数据库中查找字符串
  'apt-get', // Debian Linux发行版中的APT软件包管理工具
  'apt-key', // 管理Debian Linux系统中的软件包密钥
  'apt-sortpkgs', // Debian Linux下对软件包索引文件进行排序的工具
  'aptitude', // Debian Linux系统中软件包管理工具
  'ar', // 建立或修改备存文件，或是从备存文件中抽取文件
  'arch', // 显示当前主机的硬件架构类型
  'arj', // 用于创建和管理.arj压缩包
  'arp', // arp 命令用于显示和修改 IP 到 MAC 转换表
  'arpd', // 收集免费ARP信息
  'arping', // 通过发送ARP协议报文测试网络
  'arptables', // 管理ARP包过滤规则表
  'arpwatch', // 监听网络上ARP的记录
  'as', // 汇编语言编译器
  'at', // 在指定时间执行一个任务
  'atop', // 监控Linux系统资源与进程的工具
  'atq', // 列出当前用户的at任务列表
  'atrm', // 删除待执行任务队列中的指定任务
  'awk', // 文本和数据进行处理的编程语言
  'axel', // 多线程下载工具
  'badblocks', // 查找磁盘中损坏的区块
  'base64', // base64 编码/解码文件或标准输入输出
  'basename', // 打印目录或者文件的基本名称
  'batch', // 在系统不繁忙的时候执行定时任务
  'bc', // 算术操作精密运算工具
  'bg', // 将前台终端作业移动到后台运行
  'bind', // 显示或设置键盘按键与其相关的功能
  'blkid', // 查看块设备的文件系统类型、LABEL、UUID等信息
  'blockdev', // 从命令行调用区块设备控制程序
  'bmodinfo', // 显示给定模块的详细信息
  'break', // 结束for，while或until循环
  'builtin', // 执行bash内建命令
  'bunzip2', // 可解压缩.bz2格式的压缩文件
  'bye', // 命令用于中断FTP连线并结束程序
  'bzcat', // 不解压，直接查看指定的.bz2文件
  'bzcmp', // 比较两个压缩包中的文件
  'bzdiff', // 直接比较两个.bz2压缩包中文件的不同
  'bzgrep', // 使用正则表达式搜索.bz2压缩包中文件
  'bzip2', // 将文件压缩成bz2格式
  'bzip2recover', // 恢复被破坏的.bz2压缩包中的文件
  'bzless', // 增强.bz2压缩包查看器
  'bzmore', // 查看bzip2压缩过的文本文件的内容
  'cal', // 显示当前日历或指定日期的日历
  'cancel', // 取消已存在的打印任务
  'cdrecord', // Linux系统下光盘刻录功能命令
  'chage', // 修改帐号和密码的有效期限
  'chattr', // 用来改变文件属性
  'chcon', // 修改对象（文件）的安全上下文
  'chfn', // 用来改变finger命令显示的信息
  'chgrp', // 用来变更文件或目录的所属群组
  'chkconfig', // 检查或设置系统的各种服务
  'chpasswd', // 批量更新用户口令的工具
  'chroot', // 把根目录换成指定的目的目录
  'chsh', // 用来更换登录系统时使用的shell
  'cksum', // 检查文件的CRC是否正确
  'clear', // 清除当前屏幕终端上的任何信息
  'clock', // 用于调整 RTC 时间
  'clockdiff', // 检测两台linux主机的时间差
  'cmp', // 比较两个文件是否有差异
  'col', // 过滤控制字符
  'colrm', // 删除文件中的指定列
  'column', // 按列格式化输出文件
  'comm', // 按行比较两个已排序的文件
  'command', // 调用并执行指定的命令
  'compress', // 使用Lempress-Ziv编码压缩数据文件
  'consoletype', // 输出已连接的终端类型
  'continue', // 结束本次循环，继续执行下一个for，while或until循环
  'convertquota', // 把老的配额文件转换为新的格式
  'cp', // 将源文件或目录复制到目标文件或目录中
  'cpio', // 用来建立、还原备份档的工具程序
  'createrepo', // 创建YUM仓库
  'csplit', // 将一个大文件分割成小的碎片文件
  'cu', // 用于连接另一个系统主机
  'cupsdisable', // 停止指定的打印机
  'cupsenable', // 启动指定的打印机
  'cut', // 连接文件并打印到标准输出设备上
  'dc', // 任意精度计算器
  'dd', // 复制文件并对原文件的内容进行转换和格式化处理
  'declare', // 声明变量，设置或显示变量的值和属性
  'depmod', // 分析可载入模块的相依性
  'dhclient', // 动态获取或释放IP地址
  'dhcpd', // 运行DHCP服务器
  'dhcrelay', // 使用dhcrelay命令可以提供中继DHCP和BOOTP请求
  'diff', // 比较给定的两个文件的不同
  'diff3', // 比较3个文件不同的地方
  'diffstat', // 显示diff命令输出信息的柱状图
  'dircolors', // 置ls命令在显示目录或文件时所用的色彩
  'dirname', // 去除文件名中的非目录部分
  'dirs', // 显示目录堆栈
  'disown', // 从当前的shell中移除作业
  'dmesg', // 显示Linux系统启动信息
  'dmidecode', // 在Linux系统下获取有关硬件方面的信息
  'dnf', // 新一代的RPM软件包管理器
  'dnsdomainname', // 定义DNS系统中FQDN名称的域名
  'domainname', // 显示和设置系统的NIS域名
  'dos2unix', // 将DOS格式文本文件转换成Unix格式
  'dpkg', // Debian Linux系统上安装、创建和管理软件包
  'dpkg-deb', // Debian Linux下的软件包管理工具
  'dpkg-divert', // Debian Linux中创建并管理一个转向列表
  'dpkg-preconfigure', // Debian Linux中软件包安装之前询问问题
  'dpkg-query', // Debian Linux中软件包的查询工具
  'dpkg-reconfigure', // Debian Linux中重新配制一个已经安装的软件包
  'dpkg-split', // Debian Linux中将大软件包分割成小包
  'dpkg-statoverride', // Debian Linux中覆盖文件的所有权和模式
  'dpkg-trigger', // Debian Linux下的软件包触发器
  'dris', // 显示和清空目录堆栈中的内容
  'dstat', // 通用的系统资源统计工具
  'dump', // 用于备份ext2或者ext3文件系统
  'e2fsck', // 用于检查第二扩展文件系统的完整性
  'e2label', // 设置第二扩展文件系统的卷标
  'echo', // 输出指定的字符串或者变量
  'ed', // 单行纯文本编辑器
  'edquota', // 用于编辑指定用户或工作组磁盘配额
  'egrep', // 在文件内查找指定的字符串
  'eject', // 用来退出抽取式设备
  'elinks', // 纯文本界面的WWW浏览器
  'elm', // 纯文本邮件客户端程序
  'emacs', // 功能强大的全屏文本编辑器
  'enable', // 启动或禁用shell内建命令
  'ethtool', // 显示或修改以太网卡的配置信息
  'ex', // 启动vim编辑器的ex编辑模式
  'exec', // 调用并执行指定的命令
  'exit', // 退出当前的shell
  'expand', // 将文件的制表符转换为空白字符
  'export', // 为shell变量或函数设置导出属性
  'exportfs', // 管理NFS共享文件系统列表
  'expr', // 一款表达式计算工具
  'false', // 返回状态为失败
  'fc', // 显示历史列表中的命令或修改指定的历史命令并执行
  'fdisk', // 查看磁盘使用情况和磁盘分区
  'fg', // 将后台作业移动到前台终端运行
  'fgrep', // 为文件搜索文字字符串
  'figlet', // 字符串转为 “字画符”
  'findfs', // 标签或UUID查找文件系统
  'finger', // 用于查找并显示用户信息
  'firewall-cmd', // Linux上新用的防火墙软件，跟iptables差不多的工具
  'fishshell', // 比 bash 更好用的 shell
  'fmt', // 读取文件后优化处理并输出
  'fold', // 控制文件内容输出时所占用的屏幕宽度
  'fping', // fping检测主机是否存在
  'fsck', // fsck（英文全拼：file system check）命令检查并且试图修复文件系统中的错误
  'ftp', // 用来设置文件系统相关功能
  'ftpcount', // 显示目前已FTP登入的用户人数
  'ftpshut', // 在指定的时间关闭FTP服务器
  'ftptop', // proftpd服务器的连接状态
  'ftpwho', // 显示当前每个ftp会话信息
  'fuser', // 使用文件或文件结构识别进程
  'gcc', // 基于C/C++的编译器
  'gcov', // 测试程序的代码覆盖率的工具
  'gdb', // 功能强大的程序调试器
  'get_module', // 获取Linux内核模块的详细信息
  'getenforce', // 显示当前SELinux的应用模式，是强制、执行还是停用
  'getent', // 查询 DNS 名称服务器中的命名空间
  'getsebool', // 查询SElinux策略内各项规则的布尔值
  'git', // 是目前世界上最先进的分布式版本控制系统
  'gpasswd', // Linux下工作组文件的管理工具
  'gpg', // 可用于签名、检查、加密或解密的工具
  'gpm', // 提供文字模式下的滑鼠事件处理
  'groupadd', // 用于创建一个新的工作组
  'groupdel', // 用于删除指定的工作组
  'groupmems', // 管理用户主要组的成员
  'groupmod', // 更改群组识别码或名称
  'grpck', // 用于验证组文件的完整性
  'grpconv', // 用来开启群组的投影密码
  'grpunconv', // 用来关闭群组的投影密码
  'grub', // 多重引导程序grub的命令行shell工具
  'grub2-mkconfig', // 生成 grub.cfg 配置文件
  'grub2-set-default', // 设置系统默认的启动内核
  'gunzip', // 用来解压缩文件
  'gzexe', // 用来压缩可执行文件
  'gzip', // 用来压缩文件
  'halt', // 关闭正在运行的Linux操作系统
  'hcitool', // hcitool是一个Linux命令行工具，用于管理和调试蓝牙设备。它可以用于扫描周围的蓝牙设备、连接到蓝牙设备发送命令和数据包等
  'hdparm', // 显示与设定硬盘的参数
  'head', // 显示文件的开头部分
  'help', // 该命令是bash内建命令，用于显示bash内建命令的帮助信息
  'hexdump', // 显示文件十六进制格式
  'host', // 常用的分析域名查询工具
  'hostid', // 显示当前主机的十六进制数字标识
  'hping3', // 测试网络及主机的安全
  'htdigest', // Apache服务器内置工具
  'htpasswd', // apache服务器创建密码认证文件
  'hwclock', // 显示与设定硬件时钟
  'iconv', // 转换文件的编码方式
  'ifcfg', // 置Linux中的网络接口参数
  'ifconfig', // 配置和显示Linux系统网卡的网络参数
  'ifdown', // 禁用指定的网络接口
  'ifstat', // 统计网络接口流量状态
  'iftop', // 一款实时流量监控工具
  'ifup', // 激活指定的网络接口
  'indent', // 格式化C语言的源文件
  'info', // Linux下info格式的帮助指令
  'init', // init进程是所有Linux进程的父进程
  'inotifywait', // 异步文件系统监控机制
  'insmod', // 将给定的模块加载到内核中
  'install', // 安装或升级软件或备份数据
  'iotop', // 用来监视磁盘I/O使用状况的工具
  'ip6tables', // linux中防火墙软件
  'ip6tables-restore', // 还原ip6tables表
  'ip6tables-save', // 保存ip6tables表配置
  'ipcalc', // 简单的IP地址计算器
  'ipcrm', // 删除消息队列、信号集、或者共享内存标识
  'ipcs', // 分析消息队列共享内存和信号量
  'iperf', // 网络性能测试工具
  'iptables-restore', // 还原iptables表的配置
  'iptables-save', // 备份iptables的表配置
  'iptraf', // 实时地监视网卡流量
  'iptstate', // 显示iptables的工作状态
  'iscsiadm', // 管理iSCSI连接
  'ispell', // 检查文件中出现的拼写错误
  'jed', // 主要用于编辑代码的编辑器
  'jobs', // 显示作业的状态
  'joe', // 强大的纯文本编辑器
  'join', // 两个文件中指定栏位内容相同的行连接起来
  'jq', // 一个灵活的轻量级命令行JSON处理器
  'kernelversion', // 打印当前内核的主版本号
  'kexec', // 从当前正在运行的内核引导到一个新内核
  'killall', // 使用进程的名称来杀死一组进程
  'lastb', // 列出登入系统失败的用户相关信息
  'lastlog', // 显示系统中所有用户最近一次登录信息
  'ld', // 将目标文件连接为可执行程序
  'ldconfig', // 动态链接库管理命令
  'ldd', // 打印程序或者库文件所依赖的共享库列表
  'less', // 分屏上下翻页浏览文件内容
  'let', // 简单的计算器，执行算术表达式
  'lftp', // 优秀的文件客户端程序
  'lftpget', // 调用lftp指令下载指定的文件
  'lha', // 压缩或解压缩lzh格式文件
  'lilo', // 安装核心载入开机管理程序
  'ln', // 用来为文件创建链接
  'lnstat', // 显示Linux系统的网路状态
  'local', // 在函数内定义局部变量
  'locate', // 比 find 好用的文件查找工具
  'logger', // 在系统日志中记录相应条目
  'login', // 登录系统或切换用户身份
  'logname', // 打印当前终端登录用户的名称
  'logout', // 退出当前登录的Shell
  'logrotate', // 系统日志进行轮转、压缩和删除
  'logsave', // 将命令的输出信息保存到指定的日志文件
  'logwatch', // 可定制和可插入式的日志监视系统
  'look', // 显示文件中以指定字符串开头的任意行
  'losetup', // 设定与控制循环（loop）设备
  'lp', // 打印文件或修改排队的打印任务
  'lpadmin', // 配置CUPS套件中的打印机和类
  'lpc', // 命令行方式打印机控制程序
  'lpq', // 显示打印队列中的打印任务的状态信息
  'lpr', // 将文件发送给指定打印机进行打印
  'lprm', // 删除打印队列中的打印任务
  'lpstat', // 显示CUPS中打印机的状态信息
  'lsattr', // 查看文件的第二扩展文件系统属性
  'lsb_release', // 显示发行版本信息
  'lsusb', // 显示本机的USB设备列表信息
  'ltrace', // 用来跟踪进程调用库函数的情况
  'lvcreate', // 用于创建LVM的逻辑卷
  'lvdisplay', // 显示逻辑卷属性
  'lvextend', // 扩展逻辑卷空间
  'lvreduce', // 收缩逻辑卷空间
  'lvremove', // 删除指定LVM逻辑卷
  'lvresize', // 调整逻辑卷空间大小
  'lvscan', // 扫描逻辑卷
  'lynx', // 纯文本模式的网页浏览器
  'mail', // 命令行下发送和接收电子邮件
  'mailq', // 显示待发送的邮件队列
  'mailstat', // 显示到达的邮件状态
  'make', // GNU的工程化编译工具
  'man', // 查看Linux中的指令帮助
  'mapfile', // 从标准输入读取行并赋值到数组
  'md5sum', // 计算和校验文件报文摘要的工具程序
  'mesg', // 设置当前终端的写权限
  'mii-tool', // 配置网络设备协商方式的工具
  'mkbootdisk', // 可建立目前系统的启动盘
  'mkcert', // 用来生成自签证书的工具
  'mkdir', // 用来创建目录
  'mke2fs', // 创建磁盘分区上的“etc2/etc3”文件系统
  'mkfs', // 用于在设备上创建Linux文件系统
  'mkinitrd', // 建立要载入ramdisk的映像文件
  'mkisofs', // 建立ISO 9660映像文件
  'mknod', // 创建字符设备文件和块设备文件
  'mkswap', // 建立和设置SWAP交换分区
  'mktemp', // 创建临时文件供shell脚本使用
  'modetest', // DRM/KMS驱动程序libdrm中的模式测试工具
  'modprobe', // 自动处理可载入模块
  'more', // 显示文件内容，每次显示一屏
  'mount', // 用于挂载Linux系统外的文件
  'mpstat', // 显示各个可用CPU的状态
  'mtools', // 显示mtools支持的指令
  'mtr', // `mtr` 结合了 `traceroute` 和 `ping` 程序的功能，是一款简单的网络诊断工具
  'mv', // 用来对文件或目录重新命名
  'mysql', // MySQL服务器客户端工具
  'mysqladmin', // MySQL服务器管理客户端
  'mysqldump', // MySQL数据库中备份工具
  'mysqlimport', // 为MySQL服务器用命令行方式导入数据
  'mysqlshow', // 显示MySQL中数据库相关信息
  'named-checkzone', // 使用named-checkzone命令可以进行区域文件有效性检查和转换，必须指定区域名称和区域文件名称
  'nano', // 字符终端文本编辑器
  'nc', // nc是网络工具中的瑞士军刀
  'ncftp', // 是增强的的FTP工具
  'neofetch', // 显示带有发行徽标的系统信息的工具
  'nethogs', // 终端下的网络流量监控工具
  'newusers', // 用于批处理的方式一次创建多个命令
  'nfsstat', // 列出NFS客户端和服务器的工作状态
  'ngrep', // 方便的数据包匹配和显示工具
  'nice', // 调整程序执行的优先权等级
  'nisdomainname', // 显示主机NIS的域名
  'nl', // 为每一个文件添加行号
  'nm', // 显示二进制目标文件的符号表
  'nmap', // 网络探测和安全审核
  'nmcli', // 地址配置工具
  'nohup', // 将程序以忽略挂起信号的方式运行起来
  'nologin', // 拒绝用户登录系统
  'nproc', // 打印可用的处理器单元数量
  'ntpdate', // 使用网络计时协议（NTP）设置日期和时间
  'ntsysv', // 集中管理系统的各种服务
  'objdump', // 显示二进制文件信息
  'od', // 输出文件的八进制、十六进制等格式编码的字节
  'openssl', // 强大的安全套接字层密码库
  'pacman', // Arch Linux 系统及其衍生系统中默认使用的包管理器
  'parted', // 磁盘分区和分区大小调整工具
  'partprobe', // 不重启的情况下重读分区
  'passwd', // 用于让用户可以更改自己的密码
  'paste', // 将多个文件按列队列合并
  'patch', // 为开放源代码软件安装补丁程序
  'pathchk', // 检查文件中不可移植的部分
  'perl', // perl语言解释器
  'pfctl', // PF防火墙的配置命令
  'pgrep', // 根据用户给出的信息在当前运行进程中查找并列出符合条件的进程ID（PID）
  'php', // PHP语言的命令行接口
  'pico', // 功能强大全屏幕的文本编辑器
  'pidof', // 查找指定名称的进程的进程号ID号
  'pidstat', // 监控进程的系统资源占用情况
  'pigz', // 可以用来解压缩文件，gzip的并行实现升级版
  'ping6', // 测试主机之间网络的连通性(ipv6)
  'pip', // Python 编程语言中的包管理器，用于安装和管理第三方 Python 模块
  'pkexec', // 以其他用户身份执行命令
  'pmap', // 报告进程的内存映射关系
  'popd', // 从目录堆栈中删除目录
  'poweroff', // 关闭Linux系统，关闭记录会被写入到/var/log/wtmp日志文件中
  'ppp-off', // 关闭ppp连线
  'pr', // 将文本文件转换成适合打印的格式
  'printf', // 格式化并输出结果
  'protoize', // GNU-C代码转换为ANSI-C代码
  'prtstat', // 显示进程信息
  'pssh', // 批量管理执行
  'pstack', // 显示每个进程的栈跟踪
  'pstree', // 以树状图的方式展现进程之间的派生关系
  'pushd', // 将目录添加到目录堆栈顶部
  'pv', // 显示当前在命令行执行的命令的进度信息，管道查看器
  'pvchange', // 修改物理卷属性
  'pvck', // 检测物理卷的LVM元数据的一致性
  'pvcreate', // 将物理硬盘分区初始化为物理卷
  'pvdisplay', // 显示物理卷的属性
  'pvremove', // 删除一个存在的物理卷
  'pvs', // 输出物理卷信息报表
  'pvscan', // 扫描系统中所有硬盘的物理卷列表
  'pwck', // 用来验证系统认证文件内容和格式的完整性
  'pwconv', // 用来开启用户的投影密码
  'pwdx', // 用于显示指定进程的当前工作目录
  'pwunconv', // 用来关闭用户的投影密码
  'quota', // 显示磁盘已使用的空间与限制
  'quotacheck', // 检查磁盘的使用空间与限制
  'quotaoff', // 关闭Linux内核中指定文件系统的磁盘配额功能
  'quotaon', // 激活Linux内核中指定文件系统的磁盘配额功能
  'rcconf', // Debian Linux下的运行等级服务配置工具
  'rcp', // 使在两台Linux主机之间的文件复制操作更简单
  'read', // 从键盘读取变量值
  'readelf', // 用于显示elf格式文件的信息
  'readonly', // 标记shell变量或函数为只读
  'reboot', // 重新启动正在运行的Linux操作系统
  'reject', // 指示打印系统拒绝发往指定目标打印机的打印任务
  'rename', // 用字符串替换的方式批量改变文件名
  'renice', // 修改正在运行的进程的调度优先级
  'reposync', // 同步yum存储库到本地目录
  'repquota', // 报表的格式输出磁盘空间限制的状态
  'resize', // 命令设置终端机视窗的大小
  'restore', // 所进行的操作和dump指令相反
  'restorecon', // 恢复文件的安全上下文
  'return', // 从函数中退出并返回数值
  'rev', // 将文件内容以字符为单位反序输出
  'rexec', // 远程执行Linux系统下命令
  'rlogin', // 从当前终端登录到远程Linux主机
  'rm', // 用于删除给定的文件和目录
  'rmdir', // 用来删除空目录
  'rmmod', // 从运行的内核中移除指定的内核模块
  'route', // 显示并设置Linux中静态路由表
  'rpm', // RPM软件包的管理工具
  'rpm2cpio', // 将RPM软件包转换为cpio格式的文件
  'rpmbuild', // 创建RPM的二进制软件包和源码软件包
  'rpmdb', // 初始化和重建RPM数据库
  'rpmquery', // 从RPM数据库中查询软件包信息
  'rpmsign', // 使用RPM软件包的签名管理工具
  'rpmverify', // 验证已安装的RPM软件包的正确性
  'rsh', // 连接远程主机并执行命令
  'rsync', // 远程数据同步工具
  'runlevel', // 打印当前Linux系统的运行等级
  'sar', // 系统运行状态统计工具
  'scp', // 加密的方式在本地主机和远程主机之间复制文件
  'screen', // 用于命令行终端切换
  'script', // 记录终端会话的所有操作
  'scriptreplay', // 重新播放终端会话的所有操作
  'sed', // 功能强大的流式文本编辑器
  'seinfo', // 查询SELinux的策略提供多少相关规则
  'semanage', // 默认目录的安全上下文查询与修改
  'sendmail', // 著名电子邮件服务器
  'seq', // 以指定增量从首数开始打印数字到尾数
  'service', // 控制系统服务的实用工具
  'sesearch', // 查询SELinux策略的规则详情
  'set', // 显示或设置shell特性及shell变量
  'setfacl', // 设置文件访问控制列表
  'setpci', // 查询和配置PCI设备的使用工具
  'setsebool', // 修改SElinux策略内各项规则的布尔值
  'setsid', // 在新的会话中运行程序
  'sftp', // 交互式的文件传输程序
  'sftp-server', // sftp协议的服务器端程序
  'sh', // shell命令解释器
  'sha256sum', // 用于计算文件的 SHA-256 哈希值
  'shift', // 移动位置参数
  'shopt', // 显示和设置shell操作选项
  'showmount', // 显示NFS服务器加载的信息
  'shuf', // 产生随机的排列
  'shutdown', // 用来执行系统关机的命令
  'skill', // 向选定的进程发送信号冻结进程
  'slabtop', // 实时显示内核slab内存缓存信息
  'sleep', // 将目前动作延迟一段时间
  'slocate', // 命令查找文件或目录
  'smartmontools', // Smartmontools 是一种硬盘检测工具，通过控制和管理硬盘的SMART（Self Monitoring Analysis and Reporting Technology，自动检测分析及报告技术）技术来实现的
  'smbclient', // 交互方式访问samba服务器
  'smbpasswd', // samba用户和密码管理工具
  'sort', // 对文本文件中所有行进行排序
  'source', // 在当前Shell环境中从指定文件读取和执行命令
  'speedtest-cli', // 命令行下测试服务器外网速度
  'spell', // 对文件进行拼写检查
  'split', // 分割任意大小的文件
  'squid', // squid服务器守护进程
  'squidclient', // squid服务器的客户端管理工具
  'ssh', // openssh套件中的客户端连接工具
  'ssh-add', // 把专用密钥添加到ssh-agent的高速缓存中
  'ssh-agent', // ssh密钥管理器
  'ssh-copy-id', // 把本地的ssh公钥文件安装到远程主机对应的账户下
  'ssh-keygen', // 为ssh生成、管理和转换认证密钥
  'ssh-keyscan', // 收集主机公钥的使用工具
  'sshd', // openssh软件套件中的服务器守护进程
  'sshpass', // 免交互 SSH 登录工具
  'startx', // 用来启动X Window
  'strace', // 跟踪系统调用和信号
  'strings', // 在对象文件或二进制文件中查找可打印的字符串
  'stty', // 修改终端命令行的相关设置
  'su', // 用于切换当前用户身份到其他用户身份
  'sudo', // 以其他身份来执行命令
  'sum', // 计算文件的校验码和显示块数
  'supervisord', // 配置后台服务/常驻进程的进程管家工具
  'suspend', // 挂起shell的执行
  'swapoff', // 关闭指定的交换空间
  'swapon', // 激活Linux系统中交换空间
  'sync', // 用于强制被改变的内容立刻写入磁盘
  'sysctl', // 时动态地修改内核的运行参数
  'syslog', // 系统默认的日志守护进程
  'systool', // 显示基于总线、类和拓扑显示系统中设备的信息
  'tac', // 连接多个文件并以行为单位反向打印到标准输出
  'tailf', // 在屏幕上显示指定文件的末尾若干行内容，通常用于日志文件的跟踪输出
  'talk', // 让用户和其他用户聊天
  'tcpdump', // 一款sniffer工具，是Linux上的抓包工具，嗅探器
  'tcpreplay', // 将保存在 PCAP 文件包重新发送，用于性能或者功能测试
  'tee', // 从标准输入读取数据并重定向到标准输出和文件
  'telinit', // 切换当前正在运行系统的运行等级
  'telnet', // 登录远程主机和管理(测试ip端口是否连通)
  'tempfile', // shell中给临时文件命名
  'test', // 执行条件表达式
  'tftp', // 在本机和tftp服务器之间使用TFTP协议传输文件
  'time', // 统计给定命令所花费的总时间
  'times', // 显示进程累计时间
  'tload', // 显示系统负载状况
  'tmux', // Tmux是一个优秀的终端复用软件，类似GNU Screen，但来自于OpenBSD，采用BSD授权
  'touch', // 创建新的空文件
  'tput', // 通过terminfo数据库对终端会话进行初始化和操作
  'tr', // 将字符进行替换压缩和删除
  'tracepath', // 追踪目的主机经过的路由信息
  'trap', // 捕捉信号和其他事件并执行命令
  'true', // 返回状态为成功
  'tty', // 显示连接到当前标准输入的终端设备文件名
  'type', // 显示指定命令的类型
  'ulimit', // 控制shell程序的资源
  'umask', // 显示或设置创建文件的权限掩码
  'umount', // 用于卸载已经加载的文件系统
  'unalias', // 删除由alias设置的别名
  'unarj', // 解压缩由arj命令创建的压缩包
  'uncompress', // 用来解压.Z文件
  'unexpand', // 将文件的空白字符转换为制表符
  'uniq', // 显示或忽略重复的行
  'unlink', // 系统调用函数unlink去删除指定的文件
  'unprotoize', // 删除C语言源代码文件中的函数原型
  'unrar', // 解压rar文件命令，从 rar 压缩包中提取文件
  'unset', // 删除指定的shell变量或函数
  'updatedb', // 创建或更新slocate命令所必需的数据库文件
  'useradd', // 创建的新的系统用户
  'userdel', // 用于删除给定的用户以及与用户相关的文件
  'usermod', // 用于修改用户的基本信息
  'usernetctl', // 被允许时操作指定的网络接口
  'users', // 打印当前主机所有登陆用户的名称
  'uucico', // UUCP文件传输服务程序
  'uupick', // 命令处理传送进来的文件
  'uuto', // 将文件传送到远端的UUCP主机
  'vdfuse', // VirtualBox软件挂载VDI分区文件工具
  'vgchange', // 修改卷组属性
  'vgconvert', // 转换卷组元数据格式
  'vgcreate', // 用于创建LVM卷组
  'vgdisplay', // 显示LVM卷组的信息
  'vgextend', // 向卷组中添加物理卷
  'vgreduce', // 从卷组中删除物理卷
  'vgremove', // 用于用户删除LVM卷组
  'vgrename', // 使用vgrename命令可以重命名卷组的名称
  'vgscan', // 扫描并显示系统中的卷组
  'vi', // 功能强大的纯文本编辑器
  'volname', // 显示指定的ISO-9660格式的设备的卷名称
  'wait', // 等待进程执行完后返回
  'wall', // 向系统当前所有打开的终端上输出信息
  'watch', // 可以将命令的输出结果输出到标准输出设备，多用于周期性执行命令/定时执行命令
  'wc', // 统计文件的字节数、字数、行数
  'whatis', // 查询一个命令执行什么功能
  'whereis', // 查找二进制程序、代码等相关文件路径
  'which', // 查找并显示给定命令的绝对路径
  'write', // 向指定登录用户终端上发送信息
  'xargs', // 给其他命令传递参数的一个过滤器
  'xauth', // 显示和编辑被用于连接X服务器的认证信息
  'xclip', // 管理 X 粘贴板
  'xhost', // 制哪些X客户端能够在X服务器上显示
  'xinit', // 是Linux下X-Window系统的初始化程序
  'xlsatoms', // 列出X服务器内部所有定义的原子成分
  'xlsclients', // 列出显示器中的客户端应用程序
  'xlsfonts', // 列出X Server使用的字体
  'xrandr', // X 窗口系统配置管理工具
  'xset', // 设置X-Window系统中的用户爱好的实用工具
  'xz', // POSIX 平台开发具有高压缩率的工具
  'yes', // 重复打印指定字符串
  'ypdomainname', // 显示主机的NIS的域名
  'yum', // 基于RPM的软件包管理器
  'zcat', // 显示压缩包中文件的内容
  'zfore', // 强制为gzip格式的压缩文件添加.gz后缀
  'zip', // 可以用来解压缩文件
  'zipinfo', // 用来列出压缩文件信息
  'zipsplit', // 将较大的zip压缩包分割成各个较小的压缩包
  'znew', // 将.Z压缩包重新转化为gzip命令压缩的.gz压缩包
  // —— 增补：常见但 dbask 未收录的命令 ——
  'chronyd', 'chronyc', 'ntpq', 'readlink', 'realpath', 'visudo',
  'adduser', 'deluser', 'getfacl', 'getcap', 'setcap', 'vim',
  'nvim', 'cmake', 'g++', 'clang', 'strip', 'python3',
  'python', 'pip3', 'node', 'npm', 'npx', 'yarn',
  'pnpm', 'deno', 'bun', 'java', 'javac', 'mvn',
  'gradle', 'go', 'cargo', 'rustc', 'ruby', 'gem',
  'composer', 'lua', 'dotnet', 'apt', 'apt-cache', 'zypper',
  'snap', 'flatpak', 'podman', 'buildah', 'skopeo', 'kubectl',
  'helm', 'k3s', 'minikube', 'docker-compose', 'nerdctl', 'mkfs.ext4',
  'mkfs.xfs', 'xfs_repair', 'resize2fs', 'tune2fs', 'gdisk', 'sgdisk',
  'wipefs', 'smartctl', 'nvme', 'mdadm', 'dmsetup', 'multipath',
  'cryptsetup', 'lshw', 'findmnt', 'ionice', 'taskset', 'chrt',
  'numactl', 'timeout', 'netcat', 'socat', 'nmtui', 'iw',
  'iwlist', 'bmon', 'nload', 'vnstat', 'iperf3', 'whois',
  'auditctl', 'ausearch', 'sestatus', 'setenforce', 'nft', 'fail2ban-client',
  'certbot', 'localectl', 'loginctl', 'machinectl', 'systemd-analyze', 'systemd-run',
  'udevadm', 'udisksctl', 'dbus-send', 'busctl', 'gdbus', 'notify-send',
  'byobu', 'zellij', 'dialog', 'whiptail', 'rg', 'fd',
  'bat', 'exa', 'eza', 'lsd', 'yq', 'fzf',
  'tldr', 'btop', 'glances', 'ncdu', 'dust', 'duf',
  'procs', 'sd', 'xh', 'httpie', 'hyperfine', 'zoxide',
  'starship', 'nginx', 'apache2ctl', 'httpd', 'haproxy', 'varnish',
  'supervisorctl', 'pm2', 'psql', 'pg_dump', 'redis-cli', 'mongosh',
  'mongo', 'sqlite3', 'ansible', 'ansible-playbook', 'terraform', 'vagrant',
  'packer', 'consul', 'vault', 'nomad', 'etcdctl', 'rclone',
  'borg', 'restic', 'duplicity', 'rsnapshot', 'timeshift', 'snapper',
  'unison', 'lsyncd', 'entr', 'plocate', 'watchman', 'unix2dos',
  'recode', 'locale', 'column -t', 'tr ', 'ncal', 'factor',
  'uuidgen', 'sha1sum', 'sha512sum', 'xxd', 'truncate', 'fallocate',
  'mkfifo', 'mmv', 'vidir',
  // —— 数据库连接与客户端（仅连接/备份/探测，不含数据库内部语句）——
  // MySQL 系
  'mysql -h 127.0.0.1 -P 3306 -u root -p', // 连接本机 MySQL
  'mysql -h ', // 连接远程 MySQL（补主机后再加 -u/-p）
  'mysql -u root -p -e "SELECT VERSION();"', // 免进交互执行单条客户端命令
  'mysqldump -h ', // 远程逻辑备份
  'mysqldump -u root -p --single-transaction --routines --triggers DB > DB.sql', // 在线一致性备份
  'mysql -u root -p DB < DB.sql', // 导入 SQL 备份
  'mysqladmin -h ', // 远程管理（ping/status/processlist）
  'mysqlshow -h ', // 列出远程库表
  'mysqlcheck -u root -p --all-databases --auto-repair', // 检查修复全部表
  'mysqlsh --uri root@127.0.0.1:3306', // MySQL Shell 连接
  // PostgreSQL 系
  'psql -h 127.0.0.1 -p 5432 -U postgres -d postgres -W', // 连接本机 PostgreSQL
  'psql -h ', // 连接远程 PostgreSQL
  'pg_dump -h ', // 远程逻辑备份单个库
  'pg_dumpall -h ', // 备份整个实例（角色+全库）
  'pg_restore -h ', // 远程恢复（配合 -d 指定库）
  'pg_isready -h ', // 探测 PG 端口是否就绪
  'createdb -h ', // 远程建库
  'dropdb -h ', // 远程删库
  'psql "postgresql://user:pass@host:5432/db?sslmode=require"', // URI 加密连接
  // Redis / MongoDB / SQLite
  'redis-cli -h 127.0.0.1 -p 6379', // 连接本机 Redis
  'redis-cli -h ', // 连接远程 Redis
  'redis-cli -h 127.0.0.1 -p 6379 -a PASSWORD --no-auth-warning', // 带密码连接
  'redis-cli -h 127.0.0.1 -p 6379 --tls --cacert ca.crt', // TLS 加密连接
  'redis-benchmark -h ', // Redis 性能压测
  'redis-cli -h 127.0.0.1 --rdb dump.rdb', // 远程触发 RDB 备份拉取
  'mongosh "mongodb://127.0.0.1:27017/admin"', // 连接本机 MongoDB
  'mongosh "mongodb://user:pass@host:27017/db?authSource=admin"', // 认证连接远程 MongoDB
  'mongodump --uri="mongodb://127.0.0.1:27017" --out ./backup', // MongoDB 逻辑备份
  'mongorestore --uri="mongodb://127.0.0.1:27017" ./backup', // MongoDB 恢复
  'sqlite3 -readonly app.db', // 只读方式打开本地 SQLite 库
  // 其他数据库 / 国产数据库连接客户端
  'sqlplus user/pass@//127.0.0.1:1521/orcl', // 连接 Oracle（EZCONNECT）
  'tnsping ORCL', // 测试 Oracle TNS 监听
  'sqlcmd -S 127.0.0.1 -U sa -P PASSWORD -Q "SELECT @@VERSION"', // SQL Server 连接执行
  'clickhouse-client --host 127.0.0.1 --port 9000 --user default --password', // 连接 ClickHouse
  'influx -host 127.0.0.1 -port 8086', // 连接 InfluxDB
  'cqlsh 127.0.0.1 9042 -u cassandra -p cassandra', // 连接 Cassandra
  'disql USER/PASS@127.0.0.1:5236', // 连接达梦数据库（DM）
  'dexp USER/PASS@127.0.0.1:5236 FILE=db.dmp LOG=exp.log', // 达梦逻辑导出
  'dimp USER/PASS@127.0.0.1:5236 FILE=db.dmp LOG=imp.log', // 达梦逻辑导入
  'ksql -h 127.0.0.1 -p 54321 -U SYSTEM -d TEST -W', // 连接人大金仓 KingbaseES
  'sys_dump -h 127.0.0.1 -U SYSTEM -d TEST -f test.sql', // 金仓逻辑备份
  'gsql -h 127.0.0.1 -p 5432 -d postgres -U gaussdb -W', // 连接 GaussDB/openGauss
  'gs_dump -h 127.0.0.1 -U gaussdb -d postgres -f pg.sql', // GaussDB/openGauss 备份
  'gs_ctl status -D /data/db', // GaussDB/openGauss 实例状态
  'obclient -h 127.0.0.1 -P 2883 -u root@sys -p -A', // 连接 OceanBase sys 租户
  'obclient -h ', // 连接 OceanBase 业务租户
  // —— Kubernetes / 云原生编排 ——
  'kubectl get nodes -o wide', // 查看集群节点
  'kubectl get pods -A -o wide', // 查看全部命名空间 Pod
  'kubectl get svc -A', // 查看全部 Service
  'kubectl get deploy -A', // 查看全部 Deployment
  'kubectl get ns', // 查看命名空间
  'kubectl get ingress -A', // 查看全部 Ingress
  'kubectl get pvc,pv', // 查看存储卷申领与持久卷
  'kubectl get events -A --sort-by=.lastTimestamp', // 按时间查看集群事件
  'kubectl describe pod ', // 查看 Pod 详情（排障）
  'kubectl logs -f ', // 跟踪 Pod 日志
  'kubectl logs --previous ', // 查看上一次崩溃容器日志
  'kubectl exec -it ', // 进入 Pod 交互排查
  'kubectl top nodes', // 节点资源水位
  'kubectl top pods -A --sort-by=cpu', // 按 CPU 排序 Pod 水位
  'kubectl apply -f deploy.yaml', // 声明式应用配置
  'kubectl apply -f - <<EOF', // 从标准输入应用清单
  'kubectl delete -f deploy.yaml', // 按清单删除资源
  'kubectl rollout restart deploy/', // 重启 Deployment
  'kubectl rollout status deploy/', // 跟踪发布状态
  'kubectl rollout undo deploy/', // 回滚上一版本
  'kubectl scale deploy/ --replicas=3', // 手动扩缩容
  'kubectl port-forward svc/', // 本地端口转发到集群服务
  'kubectl cp ', // 本地与 Pod 间复制文件
  'kubectl debug -it ', // 临时调试容器（无需镜像内 shell）
  'kubectl cluster-info', // 集群入口信息
  'kubectl config get-contexts', // 查看 kubeconfig 上下文
  'kubectl config use-context ', // 切换集群上下文
  'kubectl auth can-i --list', // 查看当前账号权限
  'kubectl cordon ', // 标记节点不可调度
  'kubectl drain ', // 驱逐节点上的 Pod（维护下线）
  'kubectl uncordon ', // 恢复节点调度
  'kubectl taint nodes ', // 节点污点管理
  'kubectl api-resources', // 列出全部资源类型（含 CRD）
  'kubectl wait --for=condition=ready pod/', // 等待 Pod 就绪
  'helm list -A', // 查看全部 release
  'helm upgrade --install ', // 安装或升级 release
  'helm rollback ', // release 回滚
  'helm template ', // 本地渲染 Chart 模板
  'helm repo update', // 更新 Chart 仓库索引
  'k9s', // 终端集群管理 UI
  'kubectx', // 快速切换 kubeconfig 上下文
  'kubens', // 快速切换命名空间
  'stern ', // 多 Pod 日志聚合跟踪
  'kustomize build . | kubectl apply -f -', // Kustomize 渲染并应用
  'crictl ps -a', // 列出容器（containerd/CRI）
  'crictl logs -f ', // 查看容器运行时日志
  'crictl images', // 列出节点镜像
  'ctr -n k8s.io images ls', // 含 K8s 命名空间的镜像列表
  'crictl stats', // 容器实时资源统计
  'kubeadm token create --print-join-command', // 生成节点加入命令
  'argocd app list', // ArgoCD 应用列表
  'istioctl analyze', // Istio 网格配置诊断
  'calicoctl node status', // Calico BGP/节点状态
  // —— 云平台 / 私有云 CLI ——
  'aws configure', // 配置 AWS 访问凭证 AK/SK
  'aws sts get-caller-identity', // 验证云账号身份
  'aws s3 ls', // 列出 S3 桶
  'aws ec2 describe-instances --output table', // 查看 ECS/EC2 实例
  'aws eks update-kubeconfig --name ', // 拉取 EKS 集群凭证
  'aws logs tail /aws/ --follow', // 跟踪 CloudWatch 日志组
  'az login', // Azure 交互式登录
  'az account show', // 当前订阅
  'az vm list -o table', // 列出 Azure 虚机
  'az aks get-credentials -g ', // 拉取 AKS 集群凭证
  'gcloud auth login', // GCP 交互式登录
  'gcloud config configurations list', // 列出 GCP 配置
  'gcloud compute instances list', // 列出 GCE 虚机
  'gcloud container clusters get-credentials ', // 拉取 GKE 凭证
  'aliyun configure', // 配置阿里云 AK/SK
  'aliyun ecs DescribeInstances --output cols=InstanceName,Status rows=Instances.Instance[]', // 列出阿里云 ECS
  'ossutil ls oss://', // 列出 OSS Bucket
  'tccli configure', // 配置腾讯云 AK/SK
  'tccli cvm DescribeInstances', // 列出腾讯云 CVM
  'coscmd list', // 列出 COS 对象
  'hcloud configure', // 配置华为云 AK/SK
  'hcloud ECS NovaListServers', // 列出华为云 ECS（ECS 兼容接口）
  'obsutil ls obs://', // 列出 OBS Bucket
  'openstack server list', // 私有云虚机列表（OpenStack）
  'openstack network list', // 私有云网络列表
  'openstack image list', // 私有云镜像列表
  'openstack volume list', // 私有云云盘列表
  'nova list', // OpenStack Nova 虚机
  'neutron net-list', // OpenStack Neutron 网络
  'glance image-list', // OpenStack Glance 镜像
  's3cmd ls', // S3 兼容对象存储列表
  'mc ls ', // MinIO 客户端列桶（S3 兼容）
  'mc alias set ', // 配置 S3 兼容服务别名
  // —— 国产操作系统 / 信创生态 ——
  'cat /etc/openEuler-release', // 识别 openEuler（欧拉）版本
  'cat /etc/anolis-release', // 识别龙蜥 Anolis OS 版本
  'cat /etc/kylin-build', // 识别银河麒麟版本
  'cat /etc/uniontechos-release', // 识别统信 UOS 服务器版版本
  'cat /etc/deepin-version', // 识别深度 deepin 版本
  'uname -m', // 查看 CPU 架构（x86_64/aarch64/loongarch64/mips64el）
  'lscpu | grep -i architecture', // 查看处理器架构（鲲鹏/飞腾/龙芯）
  'kysec_get -s', // 麒麟安全机制状态（KYSEC）
  'kysec_set -n', // 设置麒麟 KYSEC 安全选项
  'getstatus', // 麒麟三权/安全状态查看
  'setstatus', // 麒麟安全运行模式切换
  'kdumpctl status', // kdump 内核转储服务状态
  'kdumpctl propagate', // 同步 kdump 内核参数（欧拉/龙蜥常用）
  'grubby --default-kernel', // 查看默认内核版本
  'grubby --info=ALL', // 列出全部内核启动项
  'dracut -f', // 重建 initramfs（内核/驱动变更后）
  'dnf module list', // 模块流列表（欧拉/龙蜥/麒麟 dnf 系）
  'yum repolist all', // 全部软件源状态（国产 OS 兼容 yum）
  'rpm -qa --queryformat "%{NAME} %{ARCH}\n" | grep aarch64', // 核查架构相关包
  'loongarch64-linux-gnu-gcc --version', // 龙芯 LoongArch 交叉工具链
  'aarch64-linux-gnu-gcc --version', // 鲲鹏/飞腾 ARM64 交叉工具链
  'docker buildx ls', // 多架构构建器（国产 CPU 镜像构建）
  'docker buildx build --platform linux/arm64,linux/amd64 -t ', // 多架构镜像构建
  'GOARCH=arm64 go build', // 交叉编译到鲲鹏/飞腾
  'GOARCH=loong64 go build', // 交叉编译到龙芯 LoongArch
  'systemctl status ndp', // 统信 UOS 网络发现服务
  'deepin-editor ', // 统信/深度图形编辑器
]

// ========== 1.5 命令语义库（由 COMMAND_CATALOG 注释自动生成，勿手改） ==========
export const CMD_SEMANTIC = [
  ["ls -la", "ls", "列出目录全部文件（含隐藏文件与权限）"],
  ["ls -lh", "ls", "以可读单位列出文件与大小"],
  ["ls -ltr", "ls", "按修改时间倒序列出文件"],
  ["ls -alh", "ls", ""],
  ["ll", "ll", ""],
  ["ls -F", "ls", ""],
  ["ls -i", "ls", ""],
  ["pwd", "pwd", ""],
  ["whoami", "whoami", ""],
  ["id", "id", ""],
  ["groups", "groups", ""],
  ["cd /etc", "cd", ""],
  ["cd /var/log", "cd", ""],
  ["cd ~", "cd", ""],
  ["cd -", "cd", ""],
  ["tree -L 2", "tree", ""],
  ["stat", "stat", ""],
  ["file", "file", ""],
  ["top", "top", "实时动态查看进程与系统资源"],
  ["htop", "htop", "增强版交互式进程查看器"],
  ["uptime", "uptime", "查看系统运行时长与平均负载"],
  ["w", "w", "查看当前登录用户及其正在执行的操作"],
  ["who", "who", ""],
  ["last -20", "last", "查看最近 20 条用户登录记录"],
  ["last reboot", "last", ""],
  ["ps aux", "ps", "列出系统全部进程"],
  ["ps -ef", "ps", ""],
  ["ps aux --sort=-%cpu | head -20", "ps", "查看 CPU 占用最高的 20 个进程"],
  ["ps aux --sort=-%mem | head -20", "ps", "查看内存占用最高的 20 个进程"],
  ["free -h", "free", "查看内存与交换分区使用情况"],
  ["free -m", "free", ""],
  ["cat /proc/meminfo", "cat", ""],
  ["vmstat 1 5", "vmstat", "每秒采样系统资源 5 次"],
  ["df -h", "df", "查看磁盘空间占用（人类可读单位）"],
  ["df -i", "df", "查看磁盘 inode 节点使用情况"],
  ["df -hT", "df", ""],
  ["du -sh *", "du", ""],
  ["du -sh /var/log", "du", ""],
  ["du -sh /* | sort -rh | head", "du", ""],
  ["iostat -xz 1 5", "iostat", ""],
  ["uname -a", "uname", "查看内核版本与系统信息"],
  ["hostname", "hostname", "查看或设置主机名"],
  ["hostnamectl", "hostnamectl", ""],
  ["cat /etc/os-release", "cat", "查看操作系统版本信息"],
  ["cat /etc/redhat-release", "cat", ""],
  ["lscpu", "lscpu", ""],
  ["lspci", "lspci", ""],
  ["lsblk", "lsblk", "查看块设备与挂载点"],
  ["lsmod", "lsmod", ""],
  ["env", "env", "查看当前环境变量"],
  ["printenv", "printenv", ""],
  ["date", "date", ""],
  ["timedatectl", "timedatectl", ""],
  ["ip addr", "ip", "查看网卡与 IP 地址"],
  ["ip route", "ip", "查看本机路由表"],
  ["ip -s link", "ip", ""],
  ["ip neigh", "ip", ""],
  ["ss -tlnp", "ss", "查看正在监听的 TCP 端口与进程"],
  ["ss -antp", "ss", "查看全部 TCP 连接状态"],
  ["ss -ulnp", "ss", ""],
  ["netstat -tlnp", "netstat", ""],
  ["netstat -i", "netstat", ""],
  ["ping -c 4", "ping", ""],
  ["traceroute", "traceroute", ""],
  ["dig +short", "dig", ""],
  ["nslookup", "nslookup", ""],
  ["curl -I", "curl", ""],
  ["curl -s", "curl", ""],
  ["curl -v", "curl", ""],
  ["wget", "wget", ""],
  ["lsof -i :80", "lsof", "查看占用 80 端口的进程"],
  ["lsof -nP | grep LISTEN", "lsof", ""],
  ["lsof -i -P -n", "lsof", ""],
  ["systemctl status nginx", "systemctl", "查看 Nginx 服务运行状态"],
  ["systemctl restart nginx", "systemctl", "重启 Nginx 服务"],
  ["systemctl reload nginx", "systemctl", ""],
  ["systemctl status mysql", "systemctl", ""],
  ["systemctl status redis", "systemctl", ""],
  ["systemctl status docker", "systemctl", ""],
  ["systemctl list-units --type=service --state=running", "systemctl", "列出正在运行的系统服务"],
  ["systemctl enable nginx", "systemctl", ""],
  ["systemctl disable nginx", "systemctl", ""],
  ["journalctl -u nginx -n 100", "journalctl", ""],
  ["journalctl -u nginx -f", "journalctl", ""],
  ["journalctl --since \"1 hour ago\"", "journalctl", ""],
  ["tail -f /var/log/nginx/access.log", "tail", ""],
  ["tail -f /var/log/nginx/error.log", "tail", ""],
  ["tail -n 100 /var/log/messages", "tail", ""],
  ["tail -n 100 /var/log/syslog", "tail", ""],
  ["cat /etc/hosts", "cat", ""],
  ["cat /etc/resolv.conf", "cat", ""],
  ["cat /etc/fstab", "cat", ""],
  ["cat /etc/passwd", "cat", ""],
  ["crontab -l", "crontab", "查看当前用户的定时任务"],
  ["iptables -L -n -v", "iptables", "查看防火墙过滤规则详情"],
  ["ufw status", "ufw", ""],
  ["docker ps", "docker", "查看正在运行的 Docker 容器"],
  ["docker ps -a", "docker", ""],
  ["docker images", "docker", "查看本地 Docker 镜像列表"],
  ["docker stats", "docker", "实时查看容器资源占用"],
  ["docker logs -f", "docker", ""],
  ["docker exec -it", "docker", ""],
  ["docker compose ps", "docker", ""],
  ["docker compose logs -f", "docker", ""],
  ["grep -rn \"\" /etc --include=\"*.conf\"", "grep", ""],
  ["find / -name \"*.conf\" 2>/dev/null", "find", ""],
  ["find . -mtime -1 -type f", "find", ""],
  ["grep -Rni \"\" /var/log --include=\"*.log\" | head -50", "grep", "在日志目录递归搜索关键词"],
  ["chmod 755", "chmod", ""],
  ["chmod +x", "chmod", ""],
  ["chown -R", "chown", ""],
  ["tar -czvf backup.tar.gz", "tar", ""],
  ["tar -xzvf", "tar", ""],
  ["unzip -o", "unzip", ""],
  ["kill -9", "kill", ""],
  ["kill -HUP", "kill", ""],
  ["pkill -f", "pkill", ""],
  ["history", "history", "查看命令执行历史"],
  ["history | grep", "history", ""],
  ["7z", "7z", "拥有极高压缩比的开源压缩软件"],
  ["ab", "ab", "Apache服务器的性能测试工具"],
  ["accept", "accept", "指示打印系统接受发往指定目标打印机的打印任务"],
  ["ack", "ack", "比grep好用的文本搜索工具"],
  ["ag", "ag", "ack 的升级版，C语言编写，更快更人性化"],
  ["alias", "alias", "定义或显示别名"],
  ["apachectl", "apachectl", "Apache服务器前端控制工具"],
  ["apk", "apk", "Alpine Linux 下的包管理工具"],
  ["apropos", "apropos", "在 whatis 数据库中查找字符串"],
  ["apt-get", "apt-get", "Debian Linux发行版中的APT软件包管理工具"],
  ["apt-key", "apt-key", "管理Debian Linux系统中的软件包密钥"],
  ["apt-sortpkgs", "apt-sortpkgs", "Debian Linux下对软件包索引文件进行排序的工具"],
  ["aptitude", "aptitude", "Debian Linux系统中软件包管理工具"],
  ["ar", "ar", "建立或修改备存文件，或是从备存文件中抽取文件"],
  ["arch", "arch", "显示当前主机的硬件架构类型"],
  ["arj", "arj", "用于创建和管理.arj压缩包"],
  ["arp", "arp", "arp 命令用于显示和修改 IP 到 MAC 转换表"],
  ["arpd", "arpd", "收集免费ARP信息"],
  ["arping", "arping", "通过发送ARP协议报文测试网络"],
  ["arptables", "arptables", "管理ARP包过滤规则表"],
  ["arpwatch", "arpwatch", "监听网络上ARP的记录"],
  ["as", "as", "汇编语言编译器"],
  ["at", "at", "在指定时间执行一个任务"],
  ["atop", "atop", "监控Linux系统资源与进程的工具"],
  ["atq", "atq", "列出当前用户的at任务列表"],
  ["atrm", "atrm", "删除待执行任务队列中的指定任务"],
  ["awk", "awk", "文本和数据进行处理的编程语言"],
  ["axel", "axel", "多线程下载工具"],
  ["badblocks", "badblocks", "查找磁盘中损坏的区块"],
  ["base64", "base64", "base64 编码/解码文件或标准输入输出"],
  ["basename", "basename", "打印目录或者文件的基本名称"],
  ["batch", "batch", "在系统不繁忙的时候执行定时任务"],
  ["bc", "bc", "算术操作精密运算工具"],
  ["bg", "bg", "将前台终端作业移动到后台运行"],
  ["bind", "bind", "显示或设置键盘按键与其相关的功能"],
  ["blkid", "blkid", "查看块设备的文件系统类型、LABEL、UUID等信息"],
  ["blockdev", "blockdev", "从命令行调用区块设备控制程序"],
  ["bmodinfo", "bmodinfo", "显示给定模块的详细信息"],
  ["break", "break", "结束for，while或until循环"],
  ["builtin", "builtin", "执行bash内建命令"],
  ["bunzip2", "bunzip2", "可解压缩.bz2格式的压缩文件"],
  ["bye", "bye", "命令用于中断FTP连线并结束程序"],
  ["bzcat", "bzcat", "不解压，直接查看指定的.bz2文件"],
  ["bzcmp", "bzcmp", "比较两个压缩包中的文件"],
  ["bzdiff", "bzdiff", "直接比较两个.bz2压缩包中文件的不同"],
  ["bzgrep", "bzgrep", "使用正则表达式搜索.bz2压缩包中文件"],
  ["bzip2", "bzip2", "将文件压缩成bz2格式"],
  ["bzip2recover", "bzip2recover", "恢复被破坏的.bz2压缩包中的文件"],
  ["bzless", "bzless", "增强.bz2压缩包查看器"],
  ["bzmore", "bzmore", "查看bzip2压缩过的文本文件的内容"],
  ["cal", "cal", "显示当前日历或指定日期的日历"],
  ["cancel", "cancel", "取消已存在的打印任务"],
  ["cdrecord", "cdrecord", "Linux系统下光盘刻录功能命令"],
  ["chage", "chage", "修改帐号和密码的有效期限"],
  ["chattr", "chattr", "用来改变文件属性"],
  ["chcon", "chcon", "修改对象（文件）的安全上下文"],
  ["chfn", "chfn", "用来改变finger命令显示的信息"],
  ["chgrp", "chgrp", "用来变更文件或目录的所属群组"],
  ["chkconfig", "chkconfig", "检查或设置系统的各种服务"],
  ["chpasswd", "chpasswd", "批量更新用户口令的工具"],
  ["chroot", "chroot", "把根目录换成指定的目的目录"],
  ["chsh", "chsh", "用来更换登录系统时使用的shell"],
  ["cksum", "cksum", "检查文件的CRC是否正确"],
  ["clear", "clear", "清除当前屏幕终端上的任何信息"],
  ["clock", "clock", "用于调整 RTC 时间"],
  ["clockdiff", "clockdiff", "检测两台linux主机的时间差"],
  ["cmp", "cmp", "比较两个文件是否有差异"],
  ["col", "col", "过滤控制字符"],
  ["colrm", "colrm", "删除文件中的指定列"],
  ["column", "column", "按列格式化输出文件"],
  ["comm", "comm", "按行比较两个已排序的文件"],
  ["command", "command", "调用并执行指定的命令"],
  ["compress", "compress", "使用Lempress-Ziv编码压缩数据文件"],
  ["consoletype", "consoletype", "输出已连接的终端类型"],
  ["continue", "continue", "结束本次循环，继续执行下一个for，while或until循环"],
  ["convertquota", "convertquota", "把老的配额文件转换为新的格式"],
  ["cp", "cp", "将源文件或目录复制到目标文件或目录中"],
  ["cpio", "cpio", "用来建立、还原备份档的工具程序"],
  ["createrepo", "createrepo", "创建YUM仓库"],
  ["csplit", "csplit", "将一个大文件分割成小的碎片文件"],
  ["cu", "cu", "用于连接另一个系统主机"],
  ["cupsdisable", "cupsdisable", "停止指定的打印机"],
  ["cupsenable", "cupsenable", "启动指定的打印机"],
  ["cut", "cut", "连接文件并打印到标准输出设备上"],
  ["dc", "dc", "任意精度计算器"],
  ["dd", "dd", "复制文件并对原文件的内容进行转换和格式化处理"],
  ["declare", "declare", "声明变量，设置或显示变量的值和属性"],
  ["depmod", "depmod", "分析可载入模块的相依性"],
  ["dhclient", "dhclient", "动态获取或释放IP地址"],
  ["dhcpd", "dhcpd", "运行DHCP服务器"],
  ["dhcrelay", "dhcrelay", "使用dhcrelay命令可以提供中继DHCP和BOOTP请求"],
  ["diff", "diff", "比较给定的两个文件的不同"],
  ["diff3", "diff3", "比较3个文件不同的地方"],
  ["diffstat", "diffstat", "显示diff命令输出信息的柱状图"],
  ["dircolors", "dircolors", "置ls命令在显示目录或文件时所用的色彩"],
  ["dirname", "dirname", "去除文件名中的非目录部分"],
  ["dirs", "dirs", "显示目录堆栈"],
  ["disown", "disown", "从当前的shell中移除作业"],
  ["dmesg", "dmesg", "显示Linux系统启动信息"],
  ["dmidecode", "dmidecode", "在Linux系统下获取有关硬件方面的信息"],
  ["dnf", "dnf", "新一代的RPM软件包管理器"],
  ["dnsdomainname", "dnsdomainname", "定义DNS系统中FQDN名称的域名"],
  ["domainname", "domainname", "显示和设置系统的NIS域名"],
  ["dos2unix", "dos2unix", "将DOS格式文本文件转换成Unix格式"],
  ["dpkg", "dpkg", "Debian Linux系统上安装、创建和管理软件包"],
  ["dpkg-deb", "dpkg-deb", "Debian Linux下的软件包管理工具"],
  ["dpkg-divert", "dpkg-divert", "Debian Linux中创建并管理一个转向列表"],
  ["dpkg-preconfigure", "dpkg-preconfigure", "Debian Linux中软件包安装之前询问问题"],
  ["dpkg-query", "dpkg-query", "Debian Linux中软件包的查询工具"],
  ["dpkg-reconfigure", "dpkg-reconfigure", "Debian Linux中重新配制一个已经安装的软件包"],
  ["dpkg-split", "dpkg-split", "Debian Linux中将大软件包分割成小包"],
  ["dpkg-statoverride", "dpkg-statoverride", "Debian Linux中覆盖文件的所有权和模式"],
  ["dpkg-trigger", "dpkg-trigger", "Debian Linux下的软件包触发器"],
  ["dris", "dris", "显示和清空目录堆栈中的内容"],
  ["dstat", "dstat", "通用的系统资源统计工具"],
  ["dump", "dump", "用于备份ext2或者ext3文件系统"],
  ["e2fsck", "e2fsck", "用于检查第二扩展文件系统的完整性"],
  ["e2label", "e2label", "设置第二扩展文件系统的卷标"],
  ["echo", "echo", "输出指定的字符串或者变量"],
  ["ed", "ed", "单行纯文本编辑器"],
  ["edquota", "edquota", "用于编辑指定用户或工作组磁盘配额"],
  ["egrep", "egrep", "在文件内查找指定的字符串"],
  ["eject", "eject", "用来退出抽取式设备"],
  ["elinks", "elinks", "纯文本界面的WWW浏览器"],
  ["elm", "elm", "纯文本邮件客户端程序"],
  ["emacs", "emacs", "功能强大的全屏文本编辑器"],
  ["enable", "enable", "启动或禁用shell内建命令"],
  ["ethtool", "ethtool", "显示或修改以太网卡的配置信息"],
  ["ex", "ex", "启动vim编辑器的ex编辑模式"],
  ["exec", "exec", "调用并执行指定的命令"],
  ["exit", "exit", "退出当前的shell"],
  ["expand", "expand", "将文件的制表符转换为空白字符"],
  ["export", "export", "为shell变量或函数设置导出属性"],
  ["exportfs", "exportfs", "管理NFS共享文件系统列表"],
  ["expr", "expr", "一款表达式计算工具"],
  ["false", "false", "返回状态为失败"],
  ["fc", "fc", "显示历史列表中的命令或修改指定的历史命令并执行"],
  ["fdisk", "fdisk", "查看磁盘使用情况和磁盘分区"],
  ["fg", "fg", "将后台作业移动到前台终端运行"],
  ["fgrep", "fgrep", "为文件搜索文字字符串"],
  ["figlet", "figlet", "字符串转为 “字画符”"],
  ["findfs", "findfs", "标签或UUID查找文件系统"],
  ["finger", "finger", "用于查找并显示用户信息"],
  ["firewall-cmd", "firewall-cmd", "Linux上新用的防火墙软件，跟iptables差不多的工具"],
  ["fishshell", "fishshell", "比 bash 更好用的 shell"],
  ["fmt", "fmt", "读取文件后优化处理并输出"],
  ["fold", "fold", "控制文件内容输出时所占用的屏幕宽度"],
  ["fping", "fping", "fping检测主机是否存在"],
  ["fsck", "fsck", "fsck（英文全拼：file system check）命令检查并且试图修复文件系统中的错误"],
  ["ftp", "ftp", "用来设置文件系统相关功能"],
  ["ftpcount", "ftpcount", "显示目前已FTP登入的用户人数"],
  ["ftpshut", "ftpshut", "在指定的时间关闭FTP服务器"],
  ["ftptop", "ftptop", "proftpd服务器的连接状态"],
  ["ftpwho", "ftpwho", "显示当前每个ftp会话信息"],
  ["fuser", "fuser", "使用文件或文件结构识别进程"],
  ["gcc", "gcc", "基于C/C++的编译器"],
  ["gcov", "gcov", "测试程序的代码覆盖率的工具"],
  ["gdb", "gdb", "功能强大的程序调试器"],
  ["get_module", "get_module", "获取Linux内核模块的详细信息"],
  ["getenforce", "getenforce", "显示当前SELinux的应用模式，是强制、执行还是停用"],
  ["getent", "getent", "查询 DNS 名称服务器中的命名空间"],
  ["getsebool", "getsebool", "查询SElinux策略内各项规则的布尔值"],
  ["git", "git", "是目前世界上最先进的分布式版本控制系统"],
  ["gpasswd", "gpasswd", "Linux下工作组文件的管理工具"],
  ["gpg", "gpg", "可用于签名、检查、加密或解密的工具"],
  ["gpm", "gpm", "提供文字模式下的滑鼠事件处理"],
  ["groupadd", "groupadd", "用于创建一个新的工作组"],
  ["groupdel", "groupdel", "用于删除指定的工作组"],
  ["groupmems", "groupmems", "管理用户主要组的成员"],
  ["groupmod", "groupmod", "更改群组识别码或名称"],
  ["grpck", "grpck", "用于验证组文件的完整性"],
  ["grpconv", "grpconv", "用来开启群组的投影密码"],
  ["grpunconv", "grpunconv", "用来关闭群组的投影密码"],
  ["grub", "grub", "多重引导程序grub的命令行shell工具"],
  ["grub2-mkconfig", "grub2-mkconfig", "生成 grub.cfg 配置文件"],
  ["grub2-set-default", "grub2-set-default", "设置系统默认的启动内核"],
  ["gunzip", "gunzip", "用来解压缩文件"],
  ["gzexe", "gzexe", "用来压缩可执行文件"],
  ["gzip", "gzip", "用来压缩文件"],
  ["halt", "halt", "关闭正在运行的Linux操作系统"],
  ["hcitool", "hcitool", "hcitool是一个Linux命令行工具，用于管理和调试蓝牙设备。它可以用于扫描周围的蓝牙设备、连接到蓝牙设备发送命令和数据包等"],
  ["hdparm", "hdparm", "显示与设定硬盘的参数"],
  ["head", "head", "显示文件的开头部分"],
  ["help", "help", "该命令是bash内建命令，用于显示bash内建命令的帮助信息"],
  ["hexdump", "hexdump", "显示文件十六进制格式"],
  ["host", "host", "常用的分析域名查询工具"],
  ["hostid", "hostid", "显示当前主机的十六进制数字标识"],
  ["hping3", "hping3", "测试网络及主机的安全"],
  ["htdigest", "htdigest", "Apache服务器内置工具"],
  ["htpasswd", "htpasswd", "apache服务器创建密码认证文件"],
  ["hwclock", "hwclock", "显示与设定硬件时钟"],
  ["iconv", "iconv", "转换文件的编码方式"],
  ["ifcfg", "ifcfg", "置Linux中的网络接口参数"],
  ["ifconfig", "ifconfig", "配置和显示Linux系统网卡的网络参数"],
  ["ifdown", "ifdown", "禁用指定的网络接口"],
  ["ifstat", "ifstat", "统计网络接口流量状态"],
  ["iftop", "iftop", "一款实时流量监控工具"],
  ["ifup", "ifup", "激活指定的网络接口"],
  ["indent", "indent", "格式化C语言的源文件"],
  ["info", "info", "Linux下info格式的帮助指令"],
  ["init", "init", "init进程是所有Linux进程的父进程"],
  ["inotifywait", "inotifywait", "异步文件系统监控机制"],
  ["insmod", "insmod", "将给定的模块加载到内核中"],
  ["install", "install", "安装或升级软件或备份数据"],
  ["iotop", "iotop", "用来监视磁盘I/O使用状况的工具"],
  ["ip6tables", "ip6tables", "linux中防火墙软件"],
  ["ip6tables-restore", "ip6tables-restore", "还原ip6tables表"],
  ["ip6tables-save", "ip6tables-save", "保存ip6tables表配置"],
  ["ipcalc", "ipcalc", "简单的IP地址计算器"],
  ["ipcrm", "ipcrm", "删除消息队列、信号集、或者共享内存标识"],
  ["ipcs", "ipcs", "分析消息队列共享内存和信号量"],
  ["iperf", "iperf", "网络性能测试工具"],
  ["iptables-restore", "iptables-restore", "还原iptables表的配置"],
  ["iptables-save", "iptables-save", "备份iptables的表配置"],
  ["iptraf", "iptraf", "实时地监视网卡流量"],
  ["iptstate", "iptstate", "显示iptables的工作状态"],
  ["iscsiadm", "iscsiadm", "管理iSCSI连接"],
  ["ispell", "ispell", "检查文件中出现的拼写错误"],
  ["jed", "jed", "主要用于编辑代码的编辑器"],
  ["jobs", "jobs", "显示作业的状态"],
  ["joe", "joe", "强大的纯文本编辑器"],
  ["join", "join", "两个文件中指定栏位内容相同的行连接起来"],
  ["jq", "jq", "一个灵活的轻量级命令行JSON处理器"],
  ["kernelversion", "kernelversion", "打印当前内核的主版本号"],
  ["kexec", "kexec", "从当前正在运行的内核引导到一个新内核"],
  ["killall", "killall", "使用进程的名称来杀死一组进程"],
  ["lastb", "lastb", "列出登入系统失败的用户相关信息"],
  ["lastlog", "lastlog", "显示系统中所有用户最近一次登录信息"],
  ["ld", "ld", "将目标文件连接为可执行程序"],
  ["ldconfig", "ldconfig", "动态链接库管理命令"],
  ["ldd", "ldd", "打印程序或者库文件所依赖的共享库列表"],
  ["less", "less", "分屏上下翻页浏览文件内容"],
  ["let", "let", "简单的计算器，执行算术表达式"],
  ["lftp", "lftp", "优秀的文件客户端程序"],
  ["lftpget", "lftpget", "调用lftp指令下载指定的文件"],
  ["lha", "lha", "压缩或解压缩lzh格式文件"],
  ["lilo", "lilo", "安装核心载入开机管理程序"],
  ["ln", "ln", "用来为文件创建链接"],
  ["lnstat", "lnstat", "显示Linux系统的网路状态"],
  ["local", "local", "在函数内定义局部变量"],
  ["locate", "locate", "比 find 好用的文件查找工具"],
  ["logger", "logger", "在系统日志中记录相应条目"],
  ["login", "login", "登录系统或切换用户身份"],
  ["logname", "logname", "打印当前终端登录用户的名称"],
  ["logout", "logout", "退出当前登录的Shell"],
  ["logrotate", "logrotate", "系统日志进行轮转、压缩和删除"],
  ["logsave", "logsave", "将命令的输出信息保存到指定的日志文件"],
  ["logwatch", "logwatch", "可定制和可插入式的日志监视系统"],
  ["look", "look", "显示文件中以指定字符串开头的任意行"],
  ["losetup", "losetup", "设定与控制循环（loop）设备"],
  ["lp", "lp", "打印文件或修改排队的打印任务"],
  ["lpadmin", "lpadmin", "配置CUPS套件中的打印机和类"],
  ["lpc", "lpc", "命令行方式打印机控制程序"],
  ["lpq", "lpq", "显示打印队列中的打印任务的状态信息"],
  ["lpr", "lpr", "将文件发送给指定打印机进行打印"],
  ["lprm", "lprm", "删除打印队列中的打印任务"],
  ["lpstat", "lpstat", "显示CUPS中打印机的状态信息"],
  ["lsattr", "lsattr", "查看文件的第二扩展文件系统属性"],
  ["lsb_release", "lsb_release", "显示发行版本信息"],
  ["lsusb", "lsusb", "显示本机的USB设备列表信息"],
  ["ltrace", "ltrace", "用来跟踪进程调用库函数的情况"],
  ["lvcreate", "lvcreate", "用于创建LVM的逻辑卷"],
  ["lvdisplay", "lvdisplay", "显示逻辑卷属性"],
  ["lvextend", "lvextend", "扩展逻辑卷空间"],
  ["lvreduce", "lvreduce", "收缩逻辑卷空间"],
  ["lvremove", "lvremove", "删除指定LVM逻辑卷"],
  ["lvresize", "lvresize", "调整逻辑卷空间大小"],
  ["lvscan", "lvscan", "扫描逻辑卷"],
  ["lynx", "lynx", "纯文本模式的网页浏览器"],
  ["mail", "mail", "命令行下发送和接收电子邮件"],
  ["mailq", "mailq", "显示待发送的邮件队列"],
  ["mailstat", "mailstat", "显示到达的邮件状态"],
  ["make", "make", "GNU的工程化编译工具"],
  ["man", "man", "查看Linux中的指令帮助"],
  ["mapfile", "mapfile", "从标准输入读取行并赋值到数组"],
  ["md5sum", "md5sum", "计算和校验文件报文摘要的工具程序"],
  ["mesg", "mesg", "设置当前终端的写权限"],
  ["mii-tool", "mii-tool", "配置网络设备协商方式的工具"],
  ["mkbootdisk", "mkbootdisk", "可建立目前系统的启动盘"],
  ["mkcert", "mkcert", "快速生成本地 HTTPS 证书"],
  ["mkdir", "mkdir", "用来创建目录"],
  ["mke2fs", "mke2fs", "创建磁盘分区上的“etc2/etc3”文件系统"],
  ["mkfs", "mkfs", "用于在设备上创建Linux文件系统"],
  ["mkinitrd", "mkinitrd", "建立要载入ramdisk的映像文件"],
  ["mkisofs", "mkisofs", "建立ISO 9660映像文件"],
  ["mknod", "mknod", "创建字符设备文件和块设备文件"],
  ["mkswap", "mkswap", "建立和设置SWAP交换分区"],
  ["mktemp", "mktemp", "创建临时文件供shell脚本使用"],
  ["modetest", "modetest", "DRM/KMS驱动程序libdrm中的模式测试工具"],
  ["modprobe", "modprobe", "自动处理可载入模块"],
  ["more", "more", "显示文件内容，每次显示一屏"],
  ["mount", "mount", "用于挂载Linux系统外的文件"],
  ["mpstat", "mpstat", "显示各个可用CPU的状态"],
  ["mtools", "mtools", "显示mtools支持的指令"],
  ["mtr", "mtr", "`mtr` 结合了 `traceroute` 和 `ping` 程序的功能，是一款简单的网络诊断工具"],
  ["mv", "mv", "用来对文件或目录重新命名"],
  ["mysql", "mysql", "MySQL服务器客户端工具"],
  ["mysqladmin", "mysqladmin", "MySQL服务器管理客户端"],
  ["mysqldump", "mysqldump", "MySQL数据库中备份工具"],
  ["mysqlimport", "mysqlimport", "为MySQL服务器用命令行方式导入数据"],
  ["mysqlshow", "mysqlshow", "显示MySQL中数据库相关信息"],
  ["named-checkzone", "named-checkzone", "使用named-checkzone命令可以进行区域文件有效性检查和转换，必须指定区域名称和区域文件名称"],
  ["nano", "nano", "字符终端文本编辑器"],
  ["nc", "nc", "nc是网络工具中的瑞士军刀"],
  ["ncftp", "ncftp", "是增强的的FTP工具"],
  ["neofetch", "neofetch", "显示带有发行徽标的系统信息的工具"],
  ["nethogs", "nethogs", "终端下的网络流量监控工具"],
  ["newusers", "newusers", "用于批处理的方式一次创建多个命令"],
  ["nfsstat", "nfsstat", "列出NFS客户端和服务器的工作状态"],
  ["ngrep", "ngrep", "方便的数据包匹配和显示工具"],
  ["nice", "nice", "调整程序执行的优先权等级"],
  ["nisdomainname", "nisdomainname", "显示主机NIS的域名"],
  ["nl", "nl", "为每一个文件添加行号"],
  ["nm", "nm", "显示二进制目标文件的符号表"],
  ["nmap", "nmap", "网络探测和安全审核"],
  ["nmcli", "nmcli", "地址配置工具"],
  ["nohup", "nohup", "将程序以忽略挂起信号的方式运行起来"],
  ["nologin", "nologin", "拒绝用户登录系统"],
  ["nproc", "nproc", "打印可用的处理器单元数量"],
  ["ntpdate", "ntpdate", "使用网络计时协议（NTP）设置日期和时间"],
  ["ntsysv", "ntsysv", "集中管理系统的各种服务"],
  ["objdump", "objdump", "显示二进制文件信息"],
  ["od", "od", "输出文件的八进制、十六进制等格式编码的字节"],
  ["openssl", "openssl", "TLS/SSL 证书与加密工具链（密码库）"],
  ["pacman", "pacman", "Arch Linux 系统及其衍生系统中默认使用的包管理器"],
  ["parted", "parted", "磁盘分区和分区大小调整工具"],
  ["partprobe", "partprobe", "不重启的情况下重读分区"],
  ["passwd", "passwd", "用于让用户可以更改自己的密码"],
  ["paste", "paste", "将多个文件按列队列合并"],
  ["patch", "patch", "为开放源代码软件安装补丁程序"],
  ["pathchk", "pathchk", "检查文件中不可移植的部分"],
  ["perl", "perl", "perl语言解释器"],
  ["pfctl", "pfctl", "PF防火墙的配置命令"],
  ["pgrep", "pgrep", "根据用户给出的信息在当前运行进程中查找并列出符合条件的进程ID（PID）"],
  ["php", "php", "PHP语言的命令行接口"],
  ["pico", "pico", "功能强大全屏幕的文本编辑器"],
  ["pidof", "pidof", "查找指定名称的进程的进程号ID号"],
  ["pidstat", "pidstat", "监控进程的系统资源占用情况"],
  ["pigz", "pigz", "可以用来解压缩文件，gzip的并行实现升级版"],
  ["ping6", "ping6", "测试主机之间网络的连通性(ipv6)"],
  ["pip", "pip", "Python 编程语言中的包管理器，用于安装和管理第三方 Python 模块"],
  ["pkexec", "pkexec", "以其他用户身份执行命令"],
  ["pmap", "pmap", "报告进程的内存映射关系"],
  ["popd", "popd", "从目录堆栈中删除目录"],
  ["poweroff", "poweroff", "关闭Linux系统，关闭记录会被写入到/var/log/wtmp日志文件中"],
  ["ppp-off", "ppp-off", "关闭ppp连线"],
  ["pr", "pr", "将文本文件转换成适合打印的格式"],
  ["printf", "printf", "格式化并输出结果"],
  ["protoize", "protoize", "GNU-C代码转换为ANSI-C代码"],
  ["prtstat", "prtstat", "显示进程信息"],
  ["pssh", "pssh", "批量管理执行"],
  ["pstack", "pstack", "显示每个进程的栈跟踪"],
  ["pstree", "pstree", "以树状图的方式展现进程之间的派生关系"],
  ["pushd", "pushd", "将目录添加到目录堆栈顶部"],
  ["pv", "pv", "显示当前在命令行执行的命令的进度信息，管道查看器"],
  ["pvchange", "pvchange", "修改物理卷属性"],
  ["pvck", "pvck", "检测物理卷的LVM元数据的一致性"],
  ["pvcreate", "pvcreate", "将物理硬盘分区初始化为物理卷"],
  ["pvdisplay", "pvdisplay", "显示物理卷的属性"],
  ["pvremove", "pvremove", "删除一个存在的物理卷"],
  ["pvs", "pvs", "输出物理卷信息报表"],
  ["pvscan", "pvscan", "扫描系统中所有硬盘的物理卷列表"],
  ["pwck", "pwck", "用来验证系统认证文件内容和格式的完整性"],
  ["pwconv", "pwconv", "用来开启用户的投影密码"],
  ["pwdx", "pwdx", "用于显示指定进程的当前工作目录"],
  ["pwunconv", "pwunconv", "用来关闭用户的投影密码"],
  ["quota", "quota", "显示磁盘已使用的空间与限制"],
  ["quotacheck", "quotacheck", "检查磁盘的使用空间与限制"],
  ["quotaoff", "quotaoff", "关闭Linux内核中指定文件系统的磁盘配额功能"],
  ["quotaon", "quotaon", "激活Linux内核中指定文件系统的磁盘配额功能"],
  ["rcconf", "rcconf", "Debian Linux下的运行等级服务配置工具"],
  ["rcp", "rcp", "使在两台Linux主机之间的文件复制操作更简单"],
  ["read", "read", "从键盘读取变量值"],
  ["readelf", "readelf", "用于显示elf格式文件的信息"],
  ["readonly", "readonly", "标记shell变量或函数为只读"],
  ["reboot", "reboot", "重新启动正在运行的Linux操作系统"],
  ["reject", "reject", "指示打印系统拒绝发往指定目标打印机的打印任务"],
  ["rename", "rename", "用字符串替换的方式批量改变文件名"],
  ["renice", "renice", "修改正在运行的进程的调度优先级"],
  ["reposync", "reposync", "同步yum存储库到本地目录"],
  ["repquota", "repquota", "报表的格式输出磁盘空间限制的状态"],
  ["resize", "resize", "命令设置终端机视窗的大小"],
  ["restore", "restore", "所进行的操作和dump指令相反"],
  ["restorecon", "restorecon", "恢复文件的安全上下文"],
  ["return", "return", "从函数中退出并返回数值"],
  ["rev", "rev", "将文件内容以字符为单位反序输出"],
  ["rexec", "rexec", "远程执行Linux系统下命令"],
  ["rlogin", "rlogin", "从当前终端登录到远程Linux主机"],
  ["rm", "rm", "用于删除给定的文件和目录"],
  ["rmdir", "rmdir", "用来删除空目录"],
  ["rmmod", "rmmod", "从运行的内核中移除指定的内核模块"],
  ["route", "route", "显示并设置Linux中静态路由表"],
  ["rpm", "rpm", "RPM软件包的管理工具"],
  ["rpm2cpio", "rpm2cpio", "将RPM软件包转换为cpio格式的文件"],
  ["rpmbuild", "rpmbuild", "创建RPM的二进制软件包和源码软件包"],
  ["rpmdb", "rpmdb", "初始化和重建RPM数据库"],
  ["rpmquery", "rpmquery", "从RPM数据库中查询软件包信息"],
  ["rpmsign", "rpmsign", "使用RPM软件包的签名管理工具"],
  ["rpmverify", "rpmverify", "验证已安装的RPM软件包的正确性"],
  ["rsh", "rsh", "连接远程主机并执行命令"],
  ["rsync", "rsync", "远程数据同步工具"],
  ["runlevel", "runlevel", "打印当前Linux系统的运行等级"],
  ["sar", "sar", "系统运行状态统计工具"],
  ["scp", "scp", "加密的方式在本地主机和远程主机之间复制文件"],
  ["screen", "screen", "用于命令行终端切换"],
  ["script", "script", "记录终端会话的所有操作"],
  ["scriptreplay", "scriptreplay", "重新播放终端会话的所有操作"],
  ["sed", "sed", "功能强大的流式文本编辑器"],
  ["seinfo", "seinfo", "查询SELinux的策略提供多少相关规则"],
  ["semanage", "semanage", "默认目录的安全上下文查询与修改"],
  ["sendmail", "sendmail", "著名电子邮件服务器"],
  ["seq", "seq", "以指定增量从首数开始打印数字到尾数"],
  ["service", "service", "控制系统服务的实用工具"],
  ["sesearch", "sesearch", "查询SELinux策略的规则详情"],
  ["set", "set", "显示或设置shell特性及shell变量"],
  ["setfacl", "setfacl", "设置文件访问控制列表"],
  ["setpci", "setpci", "查询和配置PCI设备的使用工具"],
  ["setsebool", "setsebool", "修改SElinux策略内各项规则的布尔值"],
  ["setsid", "setsid", "在新的会话中运行程序"],
  ["sftp", "sftp", "交互式的文件传输程序"],
  ["sftp-server", "sftp-server", "sftp协议的服务器端程序"],
  ["sh", "sh", "shell命令解释器"],
  ["sha256sum", "sha256sum", "用于计算文件的 SHA-256 哈希值"],
  ["shift", "shift", "移动位置参数"],
  ["shopt", "shopt", "显示和设置shell操作选项"],
  ["showmount", "showmount", "显示NFS服务器加载的信息"],
  ["shuf", "shuf", "产生随机的排列"],
  ["shutdown", "shutdown", "用来执行系统关机的命令"],
  ["skill", "skill", "向选定的进程发送信号冻结进程"],
  ["slabtop", "slabtop", "实时显示内核slab内存缓存信息"],
  ["sleep", "sleep", "将目前动作延迟一段时间"],
  ["slocate", "slocate", "命令查找文件或目录"],
  ["smartmontools", "smartmontools", "Smartmontools 是一种硬盘检测工具，通过控制和管理硬盘的SMART（Self Monitoring Analysis and Reporting Technology，自动检测分析及报告技术）技术来实现的"],
  ["smbclient", "smbclient", "交互方式访问samba服务器"],
  ["smbpasswd", "smbpasswd", "samba用户和密码管理工具"],
  ["sort", "sort", "对文本文件中所有行进行排序"],
  ["source", "source", "在当前Shell环境中从指定文件读取和执行命令"],
  ["speedtest-cli", "speedtest-cli", "命令行下测试服务器外网速度"],
  ["spell", "spell", "对文件进行拼写检查"],
  ["split", "split", "分割任意大小的文件"],
  ["squid", "squid", "squid服务器守护进程"],
  ["squidclient", "squidclient", "squid服务器的客户端管理工具"],
  ["ssh", "ssh", "openssh套件中的客户端连接工具"],
  ["ssh-add", "ssh-add", "把专用密钥添加到ssh-agent的高速缓存中"],
  ["ssh-agent", "ssh-agent", "ssh密钥管理器"],
  ["ssh-copy-id", "ssh-copy-id", "把本地的ssh公钥文件安装到远程主机对应的账户下"],
  ["ssh-keygen", "ssh-keygen", "为ssh生成、管理和转换认证密钥"],
  ["ssh-keyscan", "ssh-keyscan", "收集主机公钥的使用工具"],
  ["sshd", "sshd", "openssh软件套件中的服务器守护进程"],
  ["sshpass", "sshpass", "免交互 SSH 登录工具"],
  ["startx", "startx", "用来启动X Window"],
  ["strace", "strace", "跟踪系统调用和信号"],
  ["strings", "strings", "在对象文件或二进制文件中查找可打印的字符串"],
  ["stty", "stty", "修改终端命令行的相关设置"],
  ["su", "su", "用于切换当前用户身份到其他用户身份"],
  ["sudo", "sudo", "以其他身份来执行命令"],
  ["sum", "sum", "计算文件的校验码和显示块数"],
  ["supervisord", "supervisord", "配置后台服务/常驻进程的进程管家工具"],
  ["suspend", "suspend", "挂起shell的执行"],
  ["swapoff", "swapoff", "关闭指定的交换空间"],
  ["swapon", "swapon", "激活Linux系统中交换空间"],
  ["sync", "sync", "用于强制被改变的内容立刻写入磁盘"],
  ["sysctl", "sysctl", "时动态地修改内核的运行参数"],
  ["syslog", "syslog", "系统默认的日志守护进程"],
  ["systool", "systool", "显示基于总线、类和拓扑显示系统中设备的信息"],
  ["tac", "tac", "连接多个文件并以行为单位反向打印到标准输出"],
  ["tailf", "tailf", "在屏幕上显示指定文件的末尾若干行内容，通常用于日志文件的跟踪输出"],
  ["talk", "talk", "让用户和其他用户聊天"],
  ["tcpdump", "tcpdump", "一款sniffer工具，是Linux上的抓包工具，嗅探器"],
  ["tcpreplay", "tcpreplay", "将保存在 PCAP 文件包重新发送，用于性能或者功能测试"],
  ["tee", "tee", "从标准输入读取数据并重定向到标准输出和文件"],
  ["telinit", "telinit", "切换当前正在运行系统的运行等级"],
  ["telnet", "telnet", "登录远程主机和管理(测试ip端口是否连通)"],
  ["tempfile", "tempfile", "shell中给临时文件命名"],
  ["test", "test", "执行条件表达式"],
  ["tftp", "tftp", "在本机和tftp服务器之间使用TFTP协议传输文件"],
  ["time", "time", "统计给定命令所花费的总时间"],
  ["times", "times", "显示进程累计时间"],
  ["tload", "tload", "显示系统负载状况"],
  ["tmux", "tmux", "Tmux是一个优秀的终端复用软件，类似GNU Screen，但来自于OpenBSD，采用BSD授权"],
  ["touch", "touch", "创建新的空文件"],
  ["tput", "tput", "通过terminfo数据库对终端会话进行初始化和操作"],
  ["tr", "tr", "将字符进行替换压缩和删除"],
  ["tracepath", "tracepath", "追踪目的主机经过的路由信息"],
  ["trap", "trap", "捕捉信号和其他事件并执行命令"],
  ["true", "true", "返回状态为成功"],
  ["tty", "tty", "显示连接到当前标准输入的终端设备文件名"],
  ["type", "type", "显示指定命令的类型"],
  ["ulimit", "ulimit", "控制shell程序的资源"],
  ["umask", "umask", "显示或设置创建文件的权限掩码"],
  ["umount", "umount", "用于卸载已经加载的文件系统"],
  ["unalias", "unalias", "删除由alias设置的别名"],
  ["unarj", "unarj", "解压缩由arj命令创建的压缩包"],
  ["uncompress", "uncompress", "用来解压.Z文件"],
  ["unexpand", "unexpand", "将文件的空白字符转换为制表符"],
  ["uniq", "uniq", "显示或忽略重复的行"],
  ["unlink", "unlink", "系统调用函数unlink去删除指定的文件"],
  ["unprotoize", "unprotoize", "删除C语言源代码文件中的函数原型"],
  ["unrar", "unrar", "解压rar文件命令，从 rar 压缩包中提取文件"],
  ["unset", "unset", "删除指定的shell变量或函数"],
  ["updatedb", "updatedb", "创建或更新slocate命令所必需的数据库文件"],
  ["useradd", "useradd", "创建的新的系统用户"],
  ["userdel", "userdel", "用于删除给定的用户以及与用户相关的文件"],
  ["usermod", "usermod", "用于修改用户的基本信息"],
  ["usernetctl", "usernetctl", "被允许时操作指定的网络接口"],
  ["users", "users", "打印当前主机所有登陆用户的名称"],
  ["uucico", "uucico", "UUCP文件传输服务程序"],
  ["uupick", "uupick", "命令处理传送进来的文件"],
  ["uuto", "uuto", "将文件传送到远端的UUCP主机"],
  ["vdfuse", "vdfuse", "VirtualBox软件挂载VDI分区文件工具"],
  ["vgchange", "vgchange", "修改卷组属性"],
  ["vgconvert", "vgconvert", "转换卷组元数据格式"],
  ["vgcreate", "vgcreate", "用于创建LVM卷组"],
  ["vgdisplay", "vgdisplay", "显示LVM卷组的信息"],
  ["vgextend", "vgextend", "向卷组中添加物理卷"],
  ["vgreduce", "vgreduce", "从卷组中删除物理卷"],
  ["vgremove", "vgremove", "用于用户删除LVM卷组"],
  ["vgrename", "vgrename", "使用vgrename命令可以重命名卷组的名称"],
  ["vgscan", "vgscan", "扫描并显示系统中的卷组"],
  ["vi", "vi", "功能强大的纯文本编辑器"],
  ["volname", "volname", "显示指定的ISO-9660格式的设备的卷名称"],
  ["wait", "wait", "等待进程执行完后返回"],
  ["wall", "wall", "向系统当前所有打开的终端上输出信息"],
  ["watch", "watch", "可以将命令的输出结果输出到标准输出设备，多用于周期性执行命令/定时执行命令"],
  ["wc", "wc", "统计文件的字节数、字数、行数"],
  ["whatis", "whatis", "查询一个命令执行什么功能"],
  ["whereis", "whereis", "查找二进制程序、代码等相关文件路径"],
  ["which", "which", "查找并显示给定命令的绝对路径"],
  ["write", "write", "向指定登录用户终端上发送信息"],
  ["xargs", "xargs", "给其他命令传递参数的一个过滤器"],
  ["xauth", "xauth", "显示和编辑被用于连接X服务器的认证信息"],
  ["xclip", "xclip", "管理 X 粘贴板"],
  ["xhost", "xhost", "制哪些X客户端能够在X服务器上显示"],
  ["xinit", "xinit", "是Linux下X-Window系统的初始化程序"],
  ["xlsatoms", "xlsatoms", "列出X服务器内部所有定义的原子成分"],
  ["xlsclients", "xlsclients", "列出显示器中的客户端应用程序"],
  ["xlsfonts", "xlsfonts", "列出X Server使用的字体"],
  ["xrandr", "xrandr", "X 窗口系统配置管理工具"],
  ["xset", "xset", "设置X-Window系统中的用户爱好的实用工具"],
  ["xz", "xz", "POSIX 平台开发具有高压缩率的工具"],
  ["yes", "yes", "重复打印指定字符串"],
  ["ypdomainname", "ypdomainname", "显示主机的NIS的域名"],
  ["yum", "yum", "基于RPM的软件包管理器"],
  ["zcat", "zcat", "显示压缩包中文件的内容"],
  ["zfore", "zfore", "强制为gzip格式的压缩文件添加.gz后缀"],
  ["zip", "zip", "可以用来解压缩文件"],
  ["zipinfo", "zipinfo", "用来列出压缩文件信息"],
  ["zipsplit", "zipsplit", "将较大的zip压缩包分割成各个较小的压缩包"],
  ["znew", "znew", "将.Z压缩包重新转化为gzip命令压缩的.gz压缩包"],
  ["chronyd", "chronyd", ""],
  ["chronyc", "chronyc", ""],
  ["ntpq", "ntpq", ""],
  ["readlink", "readlink", ""],
  ["realpath", "realpath", ""],
  ["visudo", "visudo", ""],
  ["adduser", "adduser", ""],
  ["deluser", "deluser", ""],
  ["getfacl", "getfacl", ""],
  ["getcap", "getcap", ""],
  ["setcap", "setcap", ""],
  ["vim", "vim", "Vi 改进版文本编辑器"],
  ["nvim", "nvim", ""],
  ["cmake", "cmake", ""],
  ["g++", "g++", ""],
  ["clang", "clang", ""],
  ["strip", "strip", ""],
  ["python3", "python3", ""],
  ["python", "python", ""],
  ["pip3", "pip3", ""],
  ["node", "node", ""],
  ["npm", "npm", ""],
  ["npx", "npx", ""],
  ["yarn", "yarn", ""],
  ["pnpm", "pnpm", ""],
  ["deno", "deno", ""],
  ["bun", "bun", ""],
  ["java", "java", ""],
  ["javac", "javac", ""],
  ["mvn", "mvn", ""],
  ["gradle", "gradle", ""],
  ["go", "go", ""],
  ["cargo", "cargo", ""],
  ["rustc", "rustc", ""],
  ["ruby", "ruby", ""],
  ["gem", "gem", ""],
  ["composer", "composer", ""],
  ["lua", "lua", ""],
  ["dotnet", "dotnet", ""],
  ["apt", "apt", ""],
  ["apt-cache", "apt-cache", ""],
  ["zypper", "zypper", ""],
  ["snap", "snap", ""],
  ["flatpak", "flatpak", ""],
  ["podman", "podman", "无守护进程容器引擎（Docker 兼容）"],
  ["buildah", "buildah", ""],
  ["skopeo", "skopeo", ""],
  ["kubectl", "kubectl", "Kubernetes 集群管理客户端"],
  ["helm", "helm", "Kubernetes 应用包管理器"],
  ["k3s", "k3s", ""],
  ["minikube", "minikube", ""],
  ["docker-compose", "docker-compose", ""],
  ["nerdctl", "nerdctl", ""],
  ["mkfs.ext4", "mkfs.ext4", ""],
  ["mkfs.xfs", "mkfs.xfs", ""],
  ["xfs_repair", "xfs_repair", ""],
  ["resize2fs", "resize2fs", ""],
  ["tune2fs", "tune2fs", ""],
  ["gdisk", "gdisk", ""],
  ["sgdisk", "sgdisk", ""],
  ["wipefs", "wipefs", ""],
  ["smartctl", "smartctl", ""],
  ["nvme", "nvme", ""],
  ["mdadm", "mdadm", ""],
  ["dmsetup", "dmsetup", ""],
  ["multipath", "multipath", ""],
  ["cryptsetup", "cryptsetup", ""],
  ["lshw", "lshw", ""],
  ["findmnt", "findmnt", ""],
  ["ionice", "ionice", ""],
  ["taskset", "taskset", ""],
  ["chrt", "chrt", ""],
  ["numactl", "numactl", ""],
  ["timeout", "timeout", ""],
  ["netcat", "netcat", ""],
  ["socat", "socat", ""],
  ["nmtui", "nmtui", ""],
  ["iw", "iw", ""],
  ["iwlist", "iwlist", ""],
  ["bmon", "bmon", ""],
  ["nload", "nload", ""],
  ["vnstat", "vnstat", ""],
  ["iperf3", "iperf3", "网络吞吐量性能测试"],
  ["whois", "whois", ""],
  ["auditctl", "auditctl", ""],
  ["ausearch", "ausearch", ""],
  ["sestatus", "sestatus", ""],
  ["setenforce", "setenforce", ""],
  ["nft", "nft", ""],
  ["fail2ban-client", "fail2ban-client", ""],
  ["certbot", "certbot", ""],
  ["localectl", "localectl", ""],
  ["loginctl", "loginctl", ""],
  ["machinectl", "machinectl", ""],
  ["systemd-analyze", "systemd-analyze", ""],
  ["systemd-run", "systemd-run", ""],
  ["udevadm", "udevadm", ""],
  ["udisksctl", "udisksctl", ""],
  ["dbus-send", "dbus-send", ""],
  ["busctl", "busctl", ""],
  ["gdbus", "gdbus", ""],
  ["notify-send", "notify-send", ""],
  ["byobu", "byobu", ""],
  ["zellij", "zellij", ""],
  ["dialog", "dialog", ""],
  ["whiptail", "whiptail", ""],
  ["rg", "rg", ""],
  ["fd", "fd", ""],
  ["bat", "bat", ""],
  ["exa", "exa", ""],
  ["eza", "eza", ""],
  ["lsd", "lsd", ""],
  ["yq", "yq", ""],
  ["fzf", "fzf", "模糊搜索工具"],
  ["tldr", "tldr", ""],
  ["btop", "btop", ""],
  ["glances", "glances", ""],
  ["ncdu", "ncdu", "交互式磁盘占用分析"],
  ["dust", "dust", ""],
  ["duf", "duf", ""],
  ["procs", "procs", ""],
  ["sd", "sd", ""],
  ["xh", "xh", ""],
  ["httpie", "httpie", ""],
  ["hyperfine", "hyperfine", ""],
  ["zoxide", "zoxide", ""],
  ["starship", "starship", ""],
  ["nginx", "nginx", "Nginx Web 服务器相关操作"],
  ["apache2ctl", "apache2ctl", ""],
  ["httpd", "httpd", ""],
  ["haproxy", "haproxy", ""],
  ["varnish", "varnish", ""],
  ["supervisorctl", "supervisorctl", ""],
  ["pm2", "pm2", ""],
  ["psql", "psql", ""],
  ["pg_dump", "pg_dump", ""],
  ["redis-cli", "redis-cli", ""],
  ["mongosh", "mongosh", ""],
  ["mongo", "mongo", ""],
  ["sqlite3", "sqlite3", ""],
  ["ansible", "ansible", ""],
  ["ansible-playbook", "ansible-playbook", ""],
  ["terraform", "terraform", ""],
  ["vagrant", "vagrant", ""],
  ["packer", "packer", ""],
  ["consul", "consul", ""],
  ["vault", "vault", ""],
  ["nomad", "nomad", ""],
  ["etcdctl", "etcdctl", ""],
  ["rclone", "rclone", ""],
  ["borg", "borg", ""],
  ["restic", "restic", ""],
  ["duplicity", "duplicity", ""],
  ["rsnapshot", "rsnapshot", ""],
  ["timeshift", "timeshift", ""],
  ["snapper", "snapper", ""],
  ["unison", "unison", ""],
  ["lsyncd", "lsyncd", ""],
  ["entr", "entr", ""],
  ["plocate", "plocate", ""],
  ["watchman", "watchman", ""],
  ["unix2dos", "unix2dos", ""],
  ["recode", "recode", ""],
  ["locale", "locale", ""],
  ["column -t", "column", ""],
  ["ncal", "ncal", ""],
  ["factor", "factor", ""],
  ["uuidgen", "uuidgen", ""],
  ["sha1sum", "sha1sum", ""],
  ["sha512sum", "sha512sum", ""],
  ["xxd", "xxd", ""],
  ["truncate", "truncate", ""],
  ["fallocate", "fallocate", ""],
  ["mkfifo", "mkfifo", ""],
  ["mmv", "mmv", ""],
  ["vidir", "vidir", ""],
  ["mysql -h 127.0.0.1 -P 3306 -u root -p", "mysql", "连接本机 MySQL"],
  ["mysql -h", "mysql", "连接远程 MySQL（补主机后再加 -u/-p）"],
  ["mysql -u root -p -e \"SELECT VERSION();\"", "mysql", "免进交互执行单条客户端命令"],
  ["mysqldump -h", "mysqldump", "远程逻辑备份"],
  ["mysqldump -u root -p --single-transaction --routines --triggers DB > DB.sql", "mysqldump", "在线一致性备份"],
  ["mysql -u root -p DB < DB.sql", "mysql", "导入 SQL 备份"],
  ["mysqladmin -h", "mysqladmin", "远程管理（ping/status/processlist）"],
  ["mysqlshow -h", "mysqlshow", "列出远程库表"],
  ["mysqlcheck -u root -p --all-databases --auto-repair", "mysqlcheck", "检查修复全部表"],
  ["mysqlsh --uri root@127.0.0.1:3306", "mysqlsh", "MySQL Shell 连接"],
  ["psql -h 127.0.0.1 -p 5432 -U postgres -d postgres -W", "psql", "连接本机 PostgreSQL"],
  ["psql -h", "psql", "连接远程 PostgreSQL"],
  ["pg_dump -h", "pg_dump", "远程逻辑备份单个库"],
  ["pg_dumpall -h", "pg_dumpall", "备份整个实例（角色+全库）"],
  ["pg_restore -h", "pg_restore", "远程恢复（配合 -d 指定库）"],
  ["pg_isready -h", "pg_isready", "探测 PG 端口是否就绪"],
  ["createdb -h", "createdb", "远程建库"],
  ["dropdb -h", "dropdb", "远程删库"],
  ["psql \"postgresql://user:pass@host:5432/db?sslmode=require\"", "psql", "URI 加密连接"],
  ["redis-cli -h 127.0.0.1 -p 6379", "redis-cli", "连接本机 Redis"],
  ["redis-cli -h", "redis-cli", "连接远程 Redis"],
  ["redis-cli -h 127.0.0.1 -p 6379 -a PASSWORD --no-auth-warning", "redis-cli", "带密码连接"],
  ["redis-cli -h 127.0.0.1 -p 6379 --tls --cacert ca.crt", "redis-cli", "TLS 加密连接"],
  ["redis-benchmark -h", "redis-benchmark", "Redis 性能压测"],
  ["redis-cli -h 127.0.0.1 --rdb dump.rdb", "redis-cli", "远程触发 RDB 备份拉取"],
  ["mongosh \"mongodb://127.0.0.1:27017/admin\"", "mongosh", "连接本机 MongoDB"],
  ["mongosh \"mongodb://user:pass@host:27017/db?authSource=admin\"", "mongosh", "认证连接远程 MongoDB"],
  ["mongodump --uri=\"mongodb://127.0.0.1:27017\" --out ./backup", "mongodump", "MongoDB 逻辑备份"],
  ["mongorestore --uri=\"mongodb://127.0.0.1:27017\" ./backup", "mongorestore", "MongoDB 恢复"],
  ["sqlite3 -readonly app.db", "sqlite3", "只读方式打开本地 SQLite 库"],
  ["sqlplus user/pass@//127.0.0.1:1521/orcl", "sqlplus", "连接 Oracle（EZCONNECT）"],
  ["tnsping ORCL", "tnsping", "测试 Oracle TNS 监听"],
  ["sqlcmd -S 127.0.0.1 -U sa -P PASSWORD -Q \"SELECT @@VERSION\"", "sqlcmd", "SQL Server 连接执行"],
  ["clickhouse-client --host 127.0.0.1 --port 9000 --user default --password", "clickhouse-client", "连接 ClickHouse"],
  ["influx -host 127.0.0.1 -port 8086", "influx", "连接 InfluxDB"],
  ["cqlsh 127.0.0.1 9042 -u cassandra -p cassandra", "cqlsh", "连接 Cassandra"],
  ["disql USER/PASS@127.0.0.1:5236", "disql", "连接达梦数据库（DM）"],
  ["dexp USER/PASS@127.0.0.1:5236 FILE=db.dmp LOG=exp.log", "dexp", "达梦逻辑导出"],
  ["dimp USER/PASS@127.0.0.1:5236 FILE=db.dmp LOG=imp.log", "dimp", "达梦逻辑导入"],
  ["ksql -h 127.0.0.1 -p 54321 -U SYSTEM -d TEST -W", "ksql", "连接人大金仓 KingbaseES"],
  ["sys_dump -h 127.0.0.1 -U SYSTEM -d TEST -f test.sql", "sys_dump", "金仓逻辑备份"],
  ["gsql -h 127.0.0.1 -p 5432 -d postgres -U gaussdb -W", "gsql", "连接 GaussDB/openGauss"],
  ["gs_dump -h 127.0.0.1 -U gaussdb -d postgres -f pg.sql", "gs_dump", "GaussDB/openGauss 备份"],
  ["gs_ctl status -D /data/db", "gs_ctl", "GaussDB/openGauss 实例状态"],
  ["obclient -h 127.0.0.1 -P 2883 -u root@sys -p -A", "obclient", "连接 OceanBase sys 租户"],
  ["obclient -h", "obclient", "连接 OceanBase 业务租户"],
  ["kubectl get nodes -o wide", "kubectl", "查看集群节点"],
  ["kubectl get pods -A -o wide", "kubectl", "查看全部命名空间 Pod"],
  ["kubectl get svc -A", "kubectl", "查看全部 Service"],
  ["kubectl get deploy -A", "kubectl", "查看全部 Deployment"],
  ["kubectl get ns", "kubectl", "查看命名空间"],
  ["kubectl get ingress -A", "kubectl", "查看全部 Ingress"],
  ["kubectl get pvc,pv", "kubectl", "查看存储卷申领与持久卷"],
  ["kubectl get events -A --sort-by=.lastTimestamp", "kubectl", "按时间查看集群事件"],
  ["kubectl describe pod", "kubectl", "查看 Pod 详情（排障）"],
  ["kubectl logs -f", "kubectl", "跟踪 Pod 日志"],
  ["kubectl logs --previous", "kubectl", "查看上一次崩溃容器日志"],
  ["kubectl exec -it", "kubectl", "进入 Pod 交互排查"],
  ["kubectl top nodes", "kubectl", "节点资源水位"],
  ["kubectl top pods -A --sort-by=cpu", "kubectl", "按 CPU 排序 Pod 水位"],
  ["kubectl apply -f deploy.yaml", "kubectl", "声明式应用配置"],
  ["kubectl apply -f - <<EOF", "kubectl", "从标准输入应用清单"],
  ["kubectl delete -f deploy.yaml", "kubectl", "按清单删除资源"],
  ["kubectl rollout restart deploy/", "kubectl", "重启 Deployment"],
  ["kubectl rollout status deploy/", "kubectl", "跟踪发布状态"],
  ["kubectl rollout undo deploy/", "kubectl", "回滚上一版本"],
  ["kubectl scale deploy/ --replicas=3", "kubectl", "手动扩缩容"],
  ["kubectl port-forward svc/", "kubectl", "本地端口转发到集群服务"],
  ["kubectl cp", "kubectl", "本地与 Pod 间复制文件"],
  ["kubectl debug -it", "kubectl", "临时调试容器（无需镜像内 shell）"],
  ["kubectl cluster-info", "kubectl", "集群入口信息"],
  ["kubectl config get-contexts", "kubectl", "查看 kubeconfig 上下文"],
  ["kubectl config use-context", "kubectl", "切换集群上下文"],
  ["kubectl auth can-i --list", "kubectl", "查看当前账号权限"],
  ["kubectl cordon", "kubectl", "标记节点不可调度"],
  ["kubectl drain", "kubectl", "驱逐节点上的 Pod（维护下线）"],
  ["kubectl uncordon", "kubectl", "恢复节点调度"],
  ["kubectl taint nodes", "kubectl", "节点污点管理"],
  ["kubectl api-resources", "kubectl", "列出全部资源类型（含 CRD）"],
  ["kubectl wait --for=condition=ready pod/", "kubectl", "等待 Pod 就绪"],
  ["helm list -A", "helm", "查看全部 release"],
  ["helm upgrade --install", "helm", "安装或升级 release"],
  ["helm rollback", "helm", "release 回滚"],
  ["helm template", "helm", "本地渲染 Chart 模板"],
  ["helm repo update", "helm", "更新 Chart 仓库索引"],
  ["k9s", "k9s", "终端集群管理 UI"],
  ["kubectx", "kubectx", "快速切换 kubeconfig 上下文"],
  ["kubens", "kubens", "快速切换命名空间"],
  ["stern", "stern", "多 Pod 日志聚合跟踪"],
  ["kustomize build . | kubectl apply -f -", "kustomize", "Kustomize 渲染并应用"],
  ["crictl ps -a", "crictl", "列出容器（containerd/CRI）"],
  ["crictl logs -f", "crictl", "查看容器运行时日志"],
  ["crictl images", "crictl", "列出节点镜像"],
  ["ctr -n k8s.io images ls", "ctr", "含 K8s 命名空间的镜像列表"],
  ["crictl stats", "crictl", "容器实时资源统计"],
  ["kubeadm token create --print-join-command", "kubeadm", "生成节点加入命令"],
  ["argocd app list", "argocd", "ArgoCD 应用列表"],
  ["istioctl analyze", "istioctl", "Istio 网格配置诊断"],
  ["calicoctl node status", "calicoctl", "Calico BGP/节点状态"],
  ["aws configure", "aws", "配置 AWS 访问凭证 AK/SK"],
  ["aws sts get-caller-identity", "aws", "验证云账号身份"],
  ["aws s3 ls", "aws", "列出 S3 桶"],
  ["aws ec2 describe-instances --output table", "aws", "查看 ECS/EC2 实例"],
  ["aws eks update-kubeconfig --name", "aws", "拉取 EKS 集群凭证"],
  ["aws logs tail /aws/ --follow", "aws", "跟踪 CloudWatch 日志组"],
  ["az login", "az", "Azure 交互式登录"],
  ["az account show", "az", "当前订阅"],
  ["az vm list -o table", "az", "列出 Azure 虚机"],
  ["az aks get-credentials -g", "az", "拉取 AKS 集群凭证"],
  ["gcloud auth login", "gcloud", "GCP 交互式登录"],
  ["gcloud config configurations list", "gcloud", "列出 GCP 配置"],
  ["gcloud compute instances list", "gcloud", "列出 GCE 虚机"],
  ["gcloud container clusters get-credentials", "gcloud", "拉取 GKE 凭证"],
  ["aliyun configure", "aliyun", "配置阿里云 AK/SK"],
  ["aliyun ecs DescribeInstances --output cols=InstanceName,Status rows=Instances.Instance[]", "aliyun", "列出阿里云 ECS"],
  ["ossutil ls oss://", "ossutil", "列出 OSS Bucket"],
  ["tccli configure", "tccli", "配置腾讯云 AK/SK"],
  ["tccli cvm DescribeInstances", "tccli", "列出腾讯云 CVM"],
  ["coscmd list", "coscmd", "列出 COS 对象"],
  ["hcloud configure", "hcloud", "配置华为云 AK/SK"],
  ["hcloud ECS NovaListServers", "hcloud", "列出华为云 ECS（ECS 兼容接口）"],
  ["obsutil ls obs://", "obsutil", "列出 OBS Bucket"],
  ["openstack server list", "openstack", "私有云虚机列表（OpenStack）"],
  ["openstack network list", "openstack", "私有云网络列表"],
  ["openstack image list", "openstack", "私有云镜像列表"],
  ["openstack volume list", "openstack", "私有云云盘列表"],
  ["nova list", "nova", "OpenStack Nova 虚机"],
  ["neutron net-list", "neutron", "OpenStack Neutron 网络"],
  ["glance image-list", "glance", "OpenStack Glance 镜像"],
  ["s3cmd ls", "s3cmd", "S3 兼容对象存储列表"],
  ["mc ls", "mc", "MinIO 客户端列桶（S3 兼容）"],
  ["mc alias set", "mc", "配置 S3 兼容服务别名"],
  ["cat /etc/openEuler-release", "cat", "识别 openEuler（欧拉）版本"],
  ["cat /etc/anolis-release", "cat", "识别龙蜥 Anolis OS 版本"],
  ["cat /etc/kylin-build", "cat", "识别银河麒麟版本"],
  ["cat /etc/uniontechos-release", "cat", "识别统信 UOS 服务器版版本"],
  ["cat /etc/deepin-version", "cat", "识别深度 deepin 版本"],
  ["uname -m", "uname", "查看 CPU 架构（x86_64/aarch64/loongarch64/mips64el）"],
  ["lscpu | grep -i architecture", "lscpu", "查看处理器架构（鲲鹏/飞腾/龙芯）"],
  ["kysec_get -s", "kysec_get", "麒麟安全机制状态（KYSEC）"],
  ["kysec_set -n", "kysec_set", "设置麒麟 KYSEC 安全选项"],
  ["getstatus", "getstatus", "麒麟三权/安全状态查看"],
  ["setstatus", "setstatus", "麒麟安全运行模式切换"],
  ["kdumpctl status", "kdumpctl", "kdump 内核转储服务状态"],
  ["kdumpctl propagate", "kdumpctl", "同步 kdump 内核参数（欧拉/龙蜥常用）"],
  ["grubby --default-kernel", "grubby", "查看默认内核版本"],
  ["grubby --info=ALL", "grubby", "列出全部内核启动项"],
  ["dracut -f", "dracut", "重建 initramfs（内核/驱动变更后）"],
  ["dnf module list", "dnf", "模块流列表（欧拉/龙蜥/麒麟 dnf 系）"],
  ["yum repolist all", "yum", "全部软件源状态（国产 OS 兼容 yum）"],
  ["rpm -qa --queryformat \"%{NAME} %{ARCH}\\n\" | grep aarch64", "rpm", "核查架构相关包"],
  ["loongarch64-linux-gnu-gcc --version", "loongarch64-linux-gnu-gcc", "龙芯 LoongArch 交叉工具链"],
  ["aarch64-linux-gnu-gcc --version", "aarch64-linux-gnu-gcc", "鲲鹏/飞腾 ARM64 交叉工具链"],
  ["docker buildx ls", "docker", "多架构构建器（国产 CPU 镜像构建）"],
  ["docker buildx build --platform linux/arm64,linux/amd64 -t", "docker", "多架构镜像构建"],
  ["GOARCH=arm64 go build", "GOARCH=arm64", "交叉编译到鲲鹏/飞腾"],
  ["GOARCH=loong64 go build", "GOARCH=loong64", "交叉编译到龙芯 LoongArch"],
  ["systemctl status ndp", "systemctl", "统信 UOS 网络发现服务"],
  ["deepin-editor", "deepin-editor", "统信/深度图形编辑器"],
]

// 结果缓存：ghost 每帧防抖刷新，同一输入行直接复用上次扫描结果
/**
 * 命令联想：按当前已输入行过滤目录
 * @param {string} line 当前输入行（命令部分）
 * @returns {string[]} 完整候选命令（含已输入部分），按相关度排序
 */
let fcKey = null, fcVal = null
export function findCompletions(line) {
  line = (line || '').trim()
  if (!line) return []
  if (fcKey === line) return fcVal
  const lower = line.toLowerCase()
  const prefix = []
  const contains = []
  for (const c of COMMAND_CATALOG) {
    const cl = c.toLowerCase()
    if (cl === lower) continue
    if (cl.startsWith(lower)) prefix.push(c)
    else if (cl.includes(lower)) contains.push(c)
  }
  // 前缀匹配优先（相关度最高），包含匹配兜底
  fcKey = line
  fcVal = [...new Set([...prefix, ...contains])].slice(0, 8)
  return fcVal
}

// ========== 2. 自然语言意图库（离线，中文运维高频说法） ==========
// 每条意图：re 强匹配（命中即直接出卡）、kw 关键词（模糊打分，用于直接匹配/澄清候选）、
// desc/commands 静态，或 build(text) 动态生成（带参数，如 swap 大小、ping 目标）
// 命令优先选取 demo 模拟器白名单内的命令，保证演示版同样跑通

// 从中文说法中提取主机名
function hostOf(text, fallback) {
  const map = { 百度: 'baidu.com', 百度一下: 'baidu.com', 淘宝: 'taobao.com', 腾讯: 'qq.com', 阿里: 'aliyun.com' }
  for (const k in map) if (text.includes(k)) return map[k]
  const en = text.match(/([a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,})/)
  return en ? en[1] : fallback
}
// 从「增加swap到4g」这类说法中提取大小
function swapSize(text) {
  const m = text.match(/(\d+(?:\.\d+)?)\s*(g|gb|m|mb)?/i)
  if (!m) return { size: '4G', mb: 4096 }
  const n = parseFloat(m[1])
  const u = (m[2] || 'g').toLowerCase()
  return u.startsWith('m')
    ? { size: n + 'M', mb: n }
    : { size: n + 'G', mb: n * 1024 }
}

const NL_INTENTS = [
  // ---- 磁盘 ----
  { re: /(磁盘|硬盘).*(空间|占用|使用|满|容量|剩余)|查.*(磁盘|硬盘)/, kw: ['磁盘', '硬盘', '空间', '容量', '剩余', '存储', 'df'],
    desc: '检查当前服务器的磁盘空间使用情况：', commands: ['df -h'] },
  { re: /inode|文件节点/, kw: ['inode', '节点'],
    desc: '查看磁盘 inode 节点使用情况：', commands: ['df -i'] },
  { re: /(大文件|哪个目录|目录.*占用.*大|占用.*最大|磁盘.*明细|磁盘.*分布)/, kw: ['磁盘', '大文件', '目录占用', '占用最大', 'du', '磁盘明细'],
    desc: '查找磁盘占用空间最大的目录：', commands: ['du -sh /* 2>/dev/null | sort -rh | head'] },
  { re: /磁盘.*(io|读写|性能)|IO.*(压力|使用|负载)/, kw: ['磁盘', '读写', 'io压力', '磁盘性能', '磁盘io'],
    desc: '查看磁盘 IO 与设备负载：', commands: ['iostat -xz 1 3'] },
  { re: /(块设备|磁盘.*(挂载|分区|列表)|挂载.*(情况|列表)|分区.*(列表|情况))/, kw: ['磁盘', '块设备', '挂载', '磁盘分区', '分区', 'lsblk'],
    desc: '查看块设备与挂载情况：', commands: ['lsblk'] },
  // ---- 内存 / swap ----
  { re: /内存|RAM|free|剩.*多少.*内存|内存.*(使用|占用|情况)/, kw: ['内存', 'ram', 'free', '剩余内存'],
    desc: '检查当前服务器的内存使用情况：', commands: ['free -h'] },
  { re: /(增加|创建|新建|添加|设置|调整|扩容|扩大|配置).*(swap|交换)|swap.*(增加|扩容|调整|分区|设为|到\s*\d)|交换.*(增加|扩容|调整|分区)/,
    kw: ['增加swap', '创建swap', 'swapfile', '交换分区'],
    build: (text) => {
      const { size, mb } = swapSize(text)
      return {
        desc: `创建 ${size} 的 swap 交换空间并设置开机自动挂载：`,
        commands: [
          `sudo fallocate -l ${size} /swapfile || sudo dd if=/dev/zero of=/swapfile bs=1M count=${mb}`,
          'sudo chmod 600 /swapfile',
          'sudo mkswap /swapfile',
          'sudo swapon /swapfile',
          "grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab",
          'free -h',
        ],
      }
    } },
  { re: /swap.*(大小|情况|状态|多少|查看|配置)|查看.*swap|交换(分区|空间).*(情况|大小|状态|多少)/, kw: ['swap', '交换分区', '交换空间'],
    desc: '查看当前 swap 交换空间的配置与使用情况：', commands: ['swapon --show', 'free -h'] },
  { re: /内存.*(占用|高).*进程|最占内存|内存.*排行|内存.*最高/, kw: ['占内存', '内存排行', '内存占用高'],
    desc: '查看内存占用最高的进程：', commands: ['ps aux --sort=-%mem | head -20'] },
  // ---- CPU / 负载 / 进程 ----
  { re: /(CPU|处理器).*(占用|高|使用|进程|排行|排名|最多)|耗.*cpu|最占.*cpu|cpu.*(高|满)/, kw: ['cpu', '处理器', 'cpu占用', '占用高'],
    desc: '查看 CPU 占用最高的进程：', commands: ['ps aux --sort=-%cpu | head -20'] },
  { re: /进程.*(列表|情况|有哪些)|所有进程|看.*进程|查看.*进程/, kw: ['进程', 'ps', '所有进程'],
    desc: '列出当前系统全部进程：', commands: ['ps aux'] },
  { re: /(负载|load)|系统.*(卡|慢|压力)|平均负载|压力测试?/, kw: ['负载', 'load', '系统卡', '系统慢', '压力'],
    desc: '查看系统运行时长与平均负载：', commands: ['uptime'] },
  // ---- 系统信息 ----
  { re: /(什么系统|系统版本|操作系统|发行版|os版本)/, kw: ['系统版本', '操作系统', '发行版', '什么系统'],
    desc: '查看当前操作系统版本信息：', commands: ['cat /etc/os-release'] },
  { re: /内核|kernel/, kw: ['内核', 'kernel'],
    desc: '查看内核版本与系统信息：', commands: ['uname -a'] },
  { re: /主机名|hostname|机器名|电脑名/, kw: ['主机名', 'hostname', '机器名'],
    desc: '查看当前主机名称：', commands: ['hostname'] },
  { re: /(多久|运行时间|启动时间|运行了多久)|什么时候.*重启/, kw: ['运行时间', '启动时间', '多久重启'],
    desc: '查看系统已运行时间与启动情况：', commands: ['uptime'] },
  { re: /硬件|CPU信息|cpu.*型号|处理器型号|机器配置|硬件配置/, kw: ['硬件', 'cpu信息', '处理器型号', '机器配置', 'lscpu'],
    desc: '查看 CPU 硬件信息：', commands: ['lscpu'] },
  { re: /当前时间|现在.*时间|几点|系统时间|日期/, kw: ['当前时间', '系统时间', '几点', '日期'],
    desc: '查看当前系统时间：', commands: ['date'] },
  { re: /时区|时间同步|ntp|chrony/, kw: ['时区', '时间同步', 'ntp', 'chrony'],
    desc: '查看时区与时间同步状态：', commands: ['timedatectl'] },
  { re: /环境变量/, kw: ['环境变量', 'env'],
    desc: '查看当前环境变量：', commands: ['env'] },
  // ---- 网络 ----
  { re: /IP地址|ip地址|本机ip|本机IP|网卡|网络接口|ip信息/, kw: ['ip地址', '网卡', '网络接口', 'ip信息'],
    desc: '查看本机网卡与 IP 地址：', commands: ['ip addr'] },
  { re: /路由|网关.*地址|默认网关|route/, kw: ['路由', '网关', 'route'],
    desc: '查看本机路由表：', commands: ['ip route'] },
  { re: /(端口|监听).*(列表|情况|端口|占用|有哪些)|开了.*哪些端口|哪些端口|端口占用|开放端口/, kw: ['端口', '监听', 'listen', '开放端口'],
    desc: '查看当前正在监听的 TCP 端口与进程：', commands: ['ss -tlnp'] },
  { re: /网络连接|连接数|established|谁连了|连接情况/, kw: ['网络连接', '连接数', '连接情况'],
    desc: '查看当前建立的网络连接：', commands: ['ss -antp'] },
  { re: /80端口|端口.*80/, kw: ['80端口'],
    desc: '查看占用 80 端口的进程：', commands: ['lsof -i :80'] },
  { re: /ping|连通性|能不能通|通不通|网络通|测.*网络|网络质量/, kw: ['ping', '连通', '通不通', '网络质量'],
    build: (text) => {
      const host = hostOf(text, '223.5.5.5')
      return { desc: `测试到 ${host} 的网络连通性（4 个包）：`, commands: [`ping -c 4 ${host}`] }
    } },
  { re: /dns解析|域名解析|解析域名|dig|nslookup/, kw: ['dns解析', '域名解析', 'dig', 'nslookup'],
    build: (text) => {
      const host = hostOf(text, 'baidu.com')
      return { desc: `解析域名 ${host} 的 DNS 记录：`, commands: [`dig +short ${host} || nslookup ${host}`] }
    } },
  { re: /防火墙.*规则|iptables|防火墙.*(状态|配置|情况)/, kw: ['防火墙', 'iptables', '防火墙规则'],
    desc: '查看当前防火墙规则：', commands: ['iptables -L -n -v'] },
  { re: /ufw/, kw: ['ufw'],
    desc: '查看 UFW 防火墙状态：', commands: ['ufw status'] },
  { re: /hosts|域名.*映射|本地解析|本地域名/, kw: ['hosts', '域名映射', '本地解析'],
    desc: '查看本地 hosts 域名映射：', commands: ['cat /etc/hosts'] },
  { re: /dns.*配置|域名服务器|resolv/, kw: ['dns配置', '域名服务器', 'resolv'],
    desc: '查看 DNS 解析配置：', commands: ['cat /etc/resolv.conf'] },
  // ---- 登录用户 ----
  { re: /谁.*在线|登录.*用户|当前.*登录|几个人.*登录|在线用户/, kw: ['在线', '登录用户', '在线用户'],
    desc: '查看当前已登录系统的用户：', commands: ['w'] },
  { re: /登录记录|登录历史|谁登过|登陆记录|登录日志/, kw: ['登录记录', '登录历史', '谁登过'],
    desc: '查看最近的用户登录记录：', commands: ['last -20'] },
  { re: /我是谁|当前账号|我的身份|当前身份|whoami/, kw: ['whoami', '我是谁', '当前账号'],
    desc: '查看当前登录账号：', commands: ['whoami'] },
  // ---- 服务与日志 ----
  { re: /nginx.*(状态|运行|起没)|网站.*(服务|状态)|web服务/, kw: ['nginx状态', '网站服务', 'nginx'],
    desc: '查看 Nginx 服务运行状态：', commands: ['systemctl status nginx'] },
  { re: /重启.*nginx|nginx.*重启/, kw: ['重启nginx'],
    desc: '重启 Nginx 服务（会短暂中断连接）：', commands: ['systemctl restart nginx'] },
  { re: /(nginx|mysql|mariadb|redis|docker|sshd|postgresql|php[\w-]*|mongodb).*(服务)?(状态|情况)|查看.*(nginx|mysql|mariadb|redis|docker|sshd|postgresql|php[\w-]*|mongodb).*服务/,
    kw: ['服务状态'],
    build: (text) => {
      const m = text.match(/nginx|mysql|mariadb|redis|docker|sshd|postgresql|php[\w-]*|mongodb/)
      const svc = m ? m[0] : 'nginx'
      return { desc: `查看 ${svc} 服务的运行状态：`, commands: [`systemctl status ${svc}`] }
    } },
  { re: /(运行|启动).*服务|所有.*service|服务列表|正在运行.*服务|有哪些服务/, kw: ['服务列表', '运行服务', 'service'],
    desc: '列出当前正在运行的系统服务：', commands: ['systemctl list-units --type=service --state=running'] },
  { re: /开机.*(启动|自启)|自启.*服务|开机自启|开机启动项/, kw: ['开机自启', '自启动', '开机启动项', 'enabled'],
    desc: '查看已设置开机自启的服务：', commands: ['systemctl list-unit-files --state=enabled'] },
  { re: /nginx.*日志|访问日志|网站.*日志/, kw: ['nginx日志', '访问日志'],
    desc: '查看 Nginx 最近的访问日志：', commands: ['tail -n 100 /var/log/nginx/access.log'] },
  { re: /错误日志|报错.*日志|日志.*报错/, kw: ['错误日志', '报错日志'],
    desc: '查看 Nginx 错误日志：', commands: ['tail -n 100 /var/log/nginx/error.log'] },
  { re: /系统日志|系统.*消息|messages/, kw: ['系统日志', 'messages'],
    desc: '查看最近的系统日志：', commands: ['tail -n 100 /var/log/messages'] },
  { re: /定时任务|计划任务|crontab|定时脚本/, kw: ['定时任务', '计划任务', 'crontab'],
    desc: '查看当前用户的定时任务：', commands: ['crontab -l'] },
  { re: /历史命令|执行过.*命令|命令历史|history/, kw: ['历史命令', '命令历史', 'history'],
    desc: '查看最近执行过的命令历史：', commands: ['history | tail -50'] },
  // ---- 容器 ----
  { re: /docker.*(容器|列表|运行|情况)|容器.*(列表|运行|情况)|正在运行.*容器/, kw: ['docker', '容器', 'docker ps'],
    desc: '查看正在运行的 Docker 容器：', commands: ['docker ps'] },
  { re: /docker.*镜像|镜像.*(列表|情况)|本地镜像/, kw: ['docker镜像', '本地镜像', '镜像'],
    desc: '查看本地 Docker 镜像：', commands: ['docker images'] },
  { re: /docker.*日志|容器.*日志/, kw: ['docker日志', '容器日志'],
    desc: '查看第一个运行中容器的最近日志：', commands: ['docker logs --tail 100 $(docker ps -q | head -1)'] },
  // ---- 文件 / 证书 ----
  { re: /当前目录|目录.*文件|有什么文件|列出文件|看看.*文件|列一下文件/, kw: ['当前目录', '有什么文件', '列出文件', '目录文件'],
    desc: '列出当前目录下的所有文件（含隐藏文件）：', commands: ['ls -la'] },
  { re: /找文件|查找文件|搜索文件|找一下文件/, kw: ['找文件', '查找文件', '搜索文件'],
    desc: '在系统中按名称查找 .conf 配置文件：', commands: ['find / -name "*.conf" 2>/dev/null | head -30'] },
  { re: /证书|SSL|TLS|crt|pem|https证书/, kw: ['证书', 'ssl', 'tls', 'crt', 'pem'],
    desc: '检查常见目录中的 SSL/TLS 证书文件：',
    commands: ['find /etc/ssl /etc/pki /etc/nginx /etc/httpd /etc/apache2 -name "*.pem" -o -name "*.crt" -o -name "*.cert" 2>/dev/null | head -30'] },
]

// 动态意图实例化
function materialize(it, text) {
  if (it.build) return it.build(text)
  return { desc: it.desc, commands: it.commands }
}
// 关键词模糊打分（CJK 关键词 2 字即给 2 分，避免「磁盘」这类短词权重不足进不了候选）
function kwScore(text, it) {
  let s = 0
  for (const kw of it.kw || []) if (text.toLowerCase().includes(kw.toLowerCase())) s += kw.length >= 2 ? 2 : 1
  return s
}

/**
 * 本地意图匹配：强正则命中即返回；否则关键词打分，高分直接返回
 * @param {string} text 中文自然语言
 * @returns {{desc:string, commands:string[]}|null}
 */
export function matchLocalIntent(text) {
  text = (text || '').trim()
  if (!text) return null
  for (const it of NL_INTENTS) if (it.re && it.re.test(text)) return materialize(it, text)
  let best = null, bs = 0
  for (const it of NL_INTENTS) {
    const s = kwScore(text, it)
    if (s > bs) { bs = s; best = it }
  }
  if (best && bs >= 4) return materialize(best, text)
  return null
}

/**
 * 语义检索（dt.js 方式）：对「命令名 + 中文说明」扁平库做非精确全文匹配，
 * 返回相关命令候选（不要求精确命中），供澄清卡反馈给用户后二次确认。
 * 惰性建库：首次调用解析 CMD_SEMANTIC，之后 O(条数) 打分（约 700 条，亚毫秒级）。
 */
let SEM_ITEMS = null
function semItems() {
  if (!SEM_ITEMS) SEM_ITEMS = CMD_SEMANTIC.map(([c, n, d]) => ({ c, n, d }))
  return SEM_ITEMS
}

// 通用 CJK 词：出现频率高但对「要执行什么命令」没有指向性，作为 2-gram 词元时忽略
const CJK_STOP = new Set([
  '查看', '检查', '使用', '情况', '设置', '状态', '信息', '系统', '命令', '当前',
  '服务器', '一下', '怎么', '如何', '帮我', '显示', '列出', '文件', '用户', '服务',
  '管理', '相关', '全部', '所有', '现在', '一个', '这个', '那个', '操作', '配置',
])

export function searchCommands(text, limit = 4) {
  const q = (text || '').trim()
  if (!q) return []
  // 词元：先按标点/空格切分；中文长串再补 2-gram 子串，提升「查看磁盘空间」
  // 「端口占用」这类短语的召回（原先只有整串命中说明才得分，候选常常只有零星几条）
  // 高频但无语义指向的通用词（查看/使用/情况…）不参与 2-gram，避免「用户组」这类
  // 短词被泛化成 w/su 之类无关候选，牺牲精度换召回
  const toks = new Set()
  for (const seg of q.split(/[\s，。；、,:：!？?（）()'"「」|]+/)) {
    if (seg.length < 2) continue
    toks.add(seg)
    if (seg.length >= 3 && /[\u4e00-\u9fff]/.test(seg)) {
      for (let i = 0; i + 2 <= seg.length; i++) {
        const bg = seg.slice(i, i + 2)
        if (!CJK_STOP.has(bg)) toks.add(bg)
      }
    }
  }
  toks.delete(q)
  const out = []
  for (const it of semItems()) {
    const cl = it.c.toLowerCase(), nl = it.n.toLowerCase()
    let s = 0
    if (q.length >= 2) {
      if (nl === q) s += 10
      else if (nl.startsWith(q)) s += 8
      else if (cl.includes(q)) s += 4
      // 说明整串命中：出现位置越靠前说明越直接相关，加分越高
      const di = it.d.indexOf(q)
      if (di >= 0) s += 6 + Math.max(0, 2 - Math.floor(di / 6))
    }
    for (const t of toks) {
      const tl = t.toLowerCase()
      if (it.d.includes(t)) s += 3
      if (nl.includes(tl) || cl.includes(tl)) s += 2
    }
    // 阈值放宽到 3：单个中文词元命中说明即入选（如「磁盘满了」只命中「磁盘」），
    // 否则候选常年只有零星几条，用户会以为联想失灵
    if (s >= 3) out.push({ s, it })
  }
  out.sort((a, b) => b.s - a.s || a.it.n.length - b.it.n.length || a.it.d.length - b.it.d.length)
  return out.slice(0, limit).map(x => x.it)
}

/**
 * 语义检索候选（澄清卡用）：包装为意图对象
 * @returns {{desc:string, commands:string[]}[]}
 */
export function searchCandidates(text, limit = 4) {
  return searchCommands(text, limit).map(it => ({ desc: it.d || ('执行 ' + it.n), commands: [it.c] }))
}

// 判断文本是否为自然语言（含 CJK 且不像 shell 命令片段）
export function looksLikeNaturalLanguage(text) {
  return /[\u4e00-\u9fa5]/.test(text || '')
}

/**
 * 构造 AI 兜底的对话消息（要求只输出 JSON）
 */
export function buildAiIntentMessages(text) {
  return [
    {
      role: 'system',
      content: '你是 Linux 终端运维助手。根据用户的中文自然语言需求，给出要执行的 Linux 命令。'
        + '只能输出一个 JSON 对象，不要输出任何解释文字或 Markdown 代码块，格式：'
        + '{"desc":"一句话说明这条命令做什么（中文）","commands":["命令1","命令2"]}。'
        + '命令要通用、安全、可直接在主流 Linux（CentOS/Ubuntu）上执行；禁止给出 rm -rf、格式化等破坏性命令。',
    },
    { role: 'user', content: text },
  ]
}

// 从 AI 返回内容中解析 JSON（兼容模型包裹 ```json 等情况）
export function parseAiIntent(content) {
  if (!content) return null
  let s = String(content).trim()
  const fence = s.match(/```(?:json)?\s*([\s\S]*?)```/i)
  if (fence) s = fence[1].trim()
  const m = s.match(/\{[\s\S]*\}/)
  if (!m) return null
  try {
    const o = JSON.parse(m[0])
    if (o && Array.isArray(o.commands) && o.commands.length) {
      return { desc: o.desc || 'AI 建议执行以下命令：', commands: o.commands.map(String) }
    }
  } catch {}
  return null
}

// ========== 3. 确认卡绘制（写入终端缓冲，仿阿里云样式） ==========
/**
 * 绘制「执行/修改/拒绝」确认卡
 * @returns {{btnAbsLine:number, buttons:{key:string,startColumn:number,endColumn:number}[]}}
 */
export function drawConfirmCard(term, desc, commands) {
  const W = Math.min(term.cols || 80, 78)
  const INNER = W - 2
  const startAbs = term.buffer.active.baseY + term.buffer.active.cursorY

  const CY = '\x1b[2;36m' // 边框：暗青
  const RS = '\x1b[0m'
  const line = raw => term.writeln(raw)
  const border = (left, right) => CY + left + '─'.repeat(INNER) + right + RS
  // 内容着色、边框固定青色，避免颜色串到边框
  const boxLine = (raw, sgr = s => s) =>
    CY + '│' + RS + sgr(fitVisual(raw, INNER)) + RS + CY + '│' + RS

  // 长命令按显示宽度折行（内容区首列预留一个空格）
  const wrapCmd = cmd => {
    const max = INNER - 1
    if (visualWidth(cmd) <= max) return [' ' + cmd]
    const rows = []
    let rest = cmd
    let first = true
    while (rest) {
      let cut = 0, w = 0
      while (cut < rest.length && w + visualWidth(rest[cut]) <= max - (first ? 1 : 4)) {
        w += visualWidth(rest[cut]); cut++
      }
      if (cut === 0) cut = 1
      rows.push((first ? ' ' : '   ↳ ') + rest.slice(0, cut))
      rest = rest.slice(cut)
      first = false
    }
    return rows
  }

  // 第 0 行：意图说明（青色圆点）
  line('\x1b[1;36m●\x1b[0m ' + desc)
  // 第 1 行：上边框
  line(border('╭', '╮'))
  // 第 2 行：标题
  line(boxLine('是否同意执行以下命令并查看输出？', s => `\x1b[1;37m${s}\x1b[0m`))
  // 第 3+ 行：命令（黄色，超长自动折行）
  let cmdLines = 0
  for (const cmd of commands) {
    for (const row of wrapCmd(cmd)) {
      line(boxLine(row, s => `\x1b[33m${s}\x1b[0m`))
      cmdLines++
    }
  }

  // 按钮行：先构造纯文本以计算单元格区间
  const defs = [
    { key: 'run', symbol: '▶', label: '执行', color: 32 },
    { key: 'edit', symbol: '✎', label: '修改', color: 37 },
    { key: 'cancel', symbol: '✖', label: '拒绝', color: 31 },
  ]
  const wide = INNER >= visualWidth('[ ▶ 执行 (Ctrl+Enter) ]  [ ✎ 修改 (Ctrl+E) ]  [ ✖ 拒绝 (Ctrl+X) ]')
  const parts = defs.map(d => wide ? `[ ${d.symbol} ${d.label} (Ctrl+${d.key === 'run' ? 'Enter' : d.key === 'edit' ? 'E' : 'X'}) ]` : `[ ${d.symbol} ]`)
  const joined = parts.join('  ')

  // 拆成「括号段 / 间隔段」，计算每个按钮在整行中的 1 基单元格区间
  // 行内布局：第 1 列是 '│'，内容从第 2 列开始，故按钮首列 = 2 + 偏移
  const segments = joined.split(/(\[[^\]]*\])/).filter(Boolean)
  const buttons = []
  let coloredBuilt = ''
  let cursor = 0
  for (const seg of segments) {
    const def = defs.find(d => seg.includes(d.symbol))
    if (def) {
      buttons.push({
        key: def.key,
        startColumn: 2 + cursor,
        endColumn: 2 + cursor + visualWidth(seg) - 1,
      })
      coloredBuilt += `\x1b[1;${def.color}m${seg}\x1b[0m`
    } else {
      coloredBuilt += seg
    }
    cursor += visualWidth(seg)
  }

  // 绘制按钮行（SGR 不占单元格），右侧补空格填满边框
  line(CY + '│' + RS + coloredBuilt + ' '.repeat(Math.max(0, INNER - visualWidth(joined))) + CY + '│' + RS)

  // 下边框
  line(border('╰', '╯'))

  const btnAbsLine = startAbs + 3 + cmdLines // 0意图 1上边框 2标题 3..命令，之后按钮行
  return { btnAbsLine, buttons }
}

// ========== 4. 澄清提问卡（低置信度时主动追问，供候选点选/数字选择） ==========
/**
 * 绘制「我没完全理解，您是想…」选择卡
 * @param {object} term xterm 实例
 * @param {string} text 用户原始输入
 * @param {{desc:string, commands:string[]}[]} options 候选意图
 * @param {{prefix?:string, suffix?:string, cancelText?:string}} [opts] 标题前缀/后缀与取消行文案覆盖
 * @returns {{links:{idx:number,absLine:number,startColumn:number,endColumn:number}[]}}
 */
export function drawAskCard(term, text, options, opts = {}) {
  const W = Math.min(term.cols || 80, 78)
  const INNER = W - 2
  const startAbs = term.buffer.active.baseY + term.buffer.active.cursorY

  const CY = '\x1b[2;36m'
  const RS = '\x1b[0m'
  const line = raw => term.writeln(raw)
  const border = (left, right) => CY + left + '─'.repeat(INNER) + right + RS
  const boxLine = (raw, sgr = s => s) =>
    CY + '│' + RS + sgr(fitVisual(raw, INNER)) + RS + CY + '│' + RS

  // 原始问题可能很长，圆点卡标题按宽度截断
  const qPrefix = opts.prefix || ' 我没完全理解「'
  const qSuffix = opts.suffix || '」，请选择您想执行的操作：'
  const budget = W - visualWidth(qPrefix) - visualWidth(qSuffix)
  let shown = text
  if (visualWidth(shown) > budget) {
    let t = '', w = 0
    for (const ch of shown) { if (w + 2 > budget - 2) break; t += ch; w += visualWidth(ch) }
    shown = t + '…'
  }
  line('\x1b[1;36m●\x1b[0m\x1b[1;37m' + qPrefix + shown + qSuffix + RS)
  line(border('╭', '╮'))
  line(boxLine(' 输入数字 1-' + options.length + ' 选择，或直接点击；0 取消', s => `\x1b[2;37m${s}\x1b[0m`))

  const links = []
  options.forEach((o, i) => {
    const desc = o.desc.replace(/[：:]\s*$/, '')
    const head = ' ' + (i + 1) + '. '
    const maxDesc = INNER - visualWidth(head)
    const d = fitVisual(desc, maxDesc).replace(/\s+$/, '')
    const row = head + d
    line(boxLine(row, s => `\x1b[1;36m${s.slice(0, visualWidth(head))}\x1b[0m\x1b[37m${s.slice(visualWidth(head))}\x1b[0m`))
    links.push({
      idx: i,
      absLine: startAbs + 3 + i, // 0问题 1上边框 2提示 3..候选
      startColumn: 2,
      endColumn: 2 + visualWidth(row) - 1,
    })
  })

  // 取消行
  const cancelRow = opts.cancelText || ' 0. 都不是，我换个说法'
  line(boxLine(cancelRow, s => `\x1b[2;31m${s}\x1b[0m`))
  links.push({
    idx: -1,
    absLine: startAbs + 3 + options.length,
    startColumn: 2,
    endColumn: 2 + visualWidth(cancelRow) - 1,
  })
  line(border('╰', '╯'))

  return { links }
}

// ========== 5. 相似命令建议（命令未找到时，按编辑距离给出近似命令供选择） ==========

// Levenshtein 编辑距离（命令名都很短，开销可忽略）
function editDistance(a, b) {
  const m = a.length, n = b.length
  if (!m) return n
  if (!n) return m
  let prev = new Array(n + 1)
  let cur = new Array(n + 1)
  for (let j = 0; j <= n; j++) prev[j] = j
  for (let i = 1; i <= m; i++) {
    cur[0] = i
    for (let j = 1; j <= n; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      cur[j] = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost)
    }
    const t = prev; prev = cur; cur = t
  }
  return prev[n]
}

// 命令词典：基础命令名 → { base, cmd, desc }，取自语义库（附中文说明）+ 命令目录补全
let CMD_DICT = null
function cmdDict() {
  if (CMD_DICT) return CMD_DICT
  const map = new Map()
  for (const [cmd, name, desc] of CMD_SEMANTIC) {
    const base = String(name || cmd.split(/\s+/)[0] || '').trim().toLowerCase()
    if (!base || !/^[a-z0-9][a-z0-9._-]*$/i.test(base)) continue
    const cur = map.get(base)
    // 同名词条优先保留带中文说明的
    if (!cur || (!cur.desc && desc)) {
      map.set(base, { base, cmd: String(cmd).trim(), desc: desc || '' })
    }
  }
  for (const c of COMMAND_CATALOG) {
    const base = String(c).split(/\s+/)[0].trim().toLowerCase()
    if (!base || !/^[a-z0-9][a-z0-9._-]*$/i.test(base)) continue
    if (!map.has(base)) map.set(base, { base, cmd: base, desc: '' })
  }
  CMD_DICT = map
  return map
}

/**
 * 为「未找到」的命令名查找相似命令候选
 * @param {string} name 未找到的命令名（如 ll、pass、mima）
 * @param {number} [limit] 返回条数上限
 * @returns {{desc:string, commands:string[]}[]}
 */
export function suggestSimilarCommands(name, limit = 8) {
  const q = String(name || '').trim().toLowerCase()
  if (!q || !/^[a-z0-9][a-z0-9._-]*$/i.test(q)) return []
  // 长度越短越难容忍差异：避免 2 字符命令给出过多无关结果；
  // 3~6 字符放宽到 2，覆盖 gti→git 这类相邻字母调换的常见手误
  const maxDist = q.length <= 2 ? 1 : q.length <= 6 ? 2 : 3
  const scored = []
  for (const it of cmdDict().values()) {
    if (it.base === q) continue
    const d = editDistance(q, it.base)
    if (d > maxDist) continue
    // 公共前缀长度：pass→passwd 这类「少打一两个字母」的手误信号强，排序时优先
    let cp = 0
    while (cp < q.length && cp < it.base.length && q[cp] === it.base[cp]) cp++
    scored.push({ d, cp, it })
  }
  scored.sort((a, b) =>
    a.d - b.d ||
    b.cp - a.cp ||
    (b.it.desc ? 1 : 0) - (a.it.desc ? 1 : 0) ||
    a.it.base.length - b.it.base.length ||
    (a.it.base < b.it.base ? -1 : 1))
  return scored.slice(0, limit).map(x => ({
    desc: x.it.desc || ('执行 ' + x.it.base),
    commands: [x.it.cmd],
  }))
}

/**
 * 从 shell 输出中识别「命令未找到」，返回缺失的命令名（识别不出则返回 ''）
 * 兼容 bash/sh「bash: xxx: command not found」与 zsh「zsh: command not found: xxx」
 * @param {string} output 终端输出（可含此前累计的少量尾部内容）
 * @param {string} [lastToken] 本次实际输入的命令名，用于优先匹配
 */
export function detectCommandNotFound(output, lastToken) {
  const s = String(output || '')
  if (!/command not found/i.test(s)) return ''
  const tok = String(lastToken || '').trim().split(/\s+/)[0]
  if (tok && s.includes(tok)) return tok
  const m = s.match(/command not found[:\s]+([^\s\r\n]+)/i) || s.match(/([^\s:]+):\s*command not found/i)
  return m ? m[1] : ''
}
