package demo

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// SimShell 虚拟 Shell 会话状态
type SimShell struct {
	Hostname string
	Username string
	CWD      string
	rng      *rand.Rand
}

// NewSimShell 创建虚拟 Shell
func NewSimShell(hostname string) *SimShell {
	return &SimShell{
		Hostname: hostname,
		Username: "root",
		CWD:      "/root",
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Exec 执行命令，返回输出（含 ANSI 提示符）
func (s *SimShell) Exec(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	cmdName := parts[0]
	args := parts[1:]

	// 危险命令拦截
	dangerous := []string{"rm", "shutdown", "reboot", "poweroff", "halt", "useradd", "userdel", "passwd", "dd", "mkfs", "fdisk"}
	for _, d := range dangerous {
		if cmdName == d {
			return fmt.Sprintf("演示模式：命令 '%s' 已被禁用（保护演示环境）\r\n", d)
		}
	}

	// 白名单命令
	switch cmdName {
	case "ls":
		return s.cmdLS(args)
	case "pwd":
		return s.CWD + "\r\n"
	case "cd":
		return s.cmdCD(args)
	case "whoami":
		return s.Username + "\r\n"
	case "hostname":
		return s.Hostname + "\r\n"
	case "date":
		return time.Now().Format("Mon Jan 2 15:04:05 MST 2006") + "\r\n"
	case "uptime":
		return s.cmdUptime()
	case "uname":
		return s.cmdUname(args)
	case "ps":
		return s.cmdPS(args)
	case "top":
		return s.cmdTop()
	case "free":
		return s.cmdFree()
	case "df":
		return s.cmdDF()
	case "systemctl":
		return s.cmdSystemctl(args)
	case "tail":
		return s.cmdTail(args)
	case "cat":
		return s.cmdCat(args)
	case "echo":
		return strings.Join(args, " ") + "\r\n"
	case "clear":
		return "\033[2J\033[H"
	case "exit", "logout":
		return "exit\r\n"
	default:
		return fmt.Sprintf("bash: %s: command not found\r\n", cmdName)
	}
}

// Prompt 返回命令提示符
func (s *SimShell) Prompt() string {
	cwd := s.CWD
	if cwd == "/root" {
		cwd = "~"
	}
	return fmt.Sprintf("[%s@%s %s]# ", s.Username, s.Hostname, cwd)
}

// ========== 命令实现 ==========

func (s *SimShell) cmdLS(args []string) string {
	showAll := false
	longFormat := false
	path := s.CWD

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "a") {
				showAll = true
			}
			if strings.Contains(arg, "l") {
				longFormat = true
			}
		} else {
			path = arg
		}
	}

	// 虚拟文件系统
	files := s.getFiles(path)
	if files == nil {
		return fmt.Sprintf("ls: cannot access '%s': No such file or directory\r\n", path)
	}

	if !longFormat {
		// 简单列出
		var names []string
		if showAll {
			names = append(names, ".", "..")
		}
		for _, f := range files {
			names = append(names, f.Name)
		}
		return strings.Join(names, "  ") + "\r\n"
	}

	// 长格式
	var sb strings.Builder
	if showAll {
		sb.WriteString("total " + fmt.Sprintf("%d", len(files)*4) + "\r\n")
		sb.WriteString("drwx------  3 root root 4096 " + s.randomDate() + " .\r\n")
		sb.WriteString("drwxr-xr-x 18 root root 4096 " + s.randomDate() + " ..\r\n")
	}
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("%s %2d %s %s %5d %s %s\r\n",
			f.Perm, f.Links, f.Owner, f.Group, f.Size, s.randomDate(), f.Name))
	}
	return sb.String()
}

func (s *SimShell) cmdCD(args []string) string {
	if len(args) == 0 {
		s.CWD = "/root"
		return ""
	}

	path := args[0]
	if path == "~" {
		s.CWD = "/root"
		return ""
	}
	if path == ".." {
		if s.CWD != "/" {
			parts := strings.Split(s.CWD, "/")
			if len(parts) <= 2 {
				s.CWD = "/"
			} else {
				s.CWD = strings.Join(parts[:len(parts)-1], "/")
			}
		}
		return ""
	}
	if strings.HasPrefix(path, "/") {
		if s.dirExists(path) {
			s.CWD = path
		} else {
			return fmt.Sprintf("bash: cd: %s: No such file or directory\r\n", path)
		}
	} else {
		newPath := s.CWD + "/" + path
		if s.CWD == "/" {
			newPath = "/" + path
		}
		if s.dirExists(newPath) {
			s.CWD = newPath
		} else {
			return fmt.Sprintf("bash: cd: %s: No such file or directory\r\n", path)
		}
	}
	return ""
}

func (s *SimShell) cmdUptime() string {
	now := time.Now()
	uptime := fmt.Sprintf("%d days, %d:%02d", 45+s.rng.Intn(30), s.rng.Intn(24), s.rng.Intn(60))
	load := fmt.Sprintf("%.2f, %.2f, %.2f", 0.5+s.rng.Float64(), 0.4+s.rng.Float64(), 0.3+s.rng.Float64())
	users := 1 + s.rng.Intn(3)
	return fmt.Sprintf(" %s up %s, %d user%s, load average: %s\r\n",
		now.Format("15:04:05"), uptime, users, plural(users), load)
}

func (s *SimShell) cmdUname(args []string) string {
	if len(args) > 0 && args[0] == "-a" {
		return fmt.Sprintf("Linux %s 5.15.0-91-generic #101-Ubuntu SMP Tue Nov 14 13:30:08 UTC 2023 x86_64 x86_64 x86_64 GNU/Linux\r\n", s.Hostname)
	}
	return "Linux\r\n"
}

func (s *SimShell) cmdPS(args []string) string {
	aux := len(args) > 0 && (args[0] == "aux" || args[0] == "-aux")

	if !aux {
		// 简单格式
		return "  PID TTY          TIME CMD\r\n" +
			fmt.Sprintf(" %4d pts/0    00:00:00 bash\r\n", 1000+s.rng.Intn(9000)) +
			fmt.Sprintf(" %4d pts/0    00:00:00 ps\r\n", 1000+s.rng.Intn(9000))
	}

	// aux 格式
	procs := []struct {
		user string
		cmd  string
		cpu  float64
		mem  float64
	}{
		{"root", "/usr/sbin/sshd -D", 0.1, 0.3},
		{"root", "/usr/sbin/nginx -g daemon off;", 0.5, 1.2},
		{"www-data", "nginx: worker process", 2.3, 2.8},
		{"mysql", "/usr/sbin/mysqld", 5.2, 15.4},
		{"redis", "/usr/bin/redis-server 127.0.0.1:6379", 1.1, 0.8},
		{"root", "/usr/lib/systemd/systemd", 0.0, 0.5},
	}

	var sb strings.Builder
	sb.WriteString("USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\r\n")
	for _, p := range procs {
		pid := 500 + s.rng.Intn(9000)
		cpu := p.cpu + s.rng.Float64()*0.5
		mem := p.mem + s.rng.Float64()*0.3
		sb.WriteString(fmt.Sprintf("%-8s %5d %4.1f %4.1f %6d %5d ?        Ss   %s   %s %s\r\n",
			p.user, pid, cpu, mem, 10000+s.rng.Intn(90000), 1000+s.rng.Intn(9000),
			s.randomTime(), s.randomDuration(), p.cmd))
	}
	return sb.String()
}

func (s *SimShell) cmdTop() string {
	// 返回一次性 top 输出（非交互式）
	load := fmt.Sprintf("%.2f, %.2f, %.2f", 0.5+s.rng.Float64(), 0.4+s.rng.Float64(), 0.3+s.rng.Float64())
	cpuUs := 5 + s.rng.Float64()*10
	cpuSy := 2 + s.rng.Float64()*5
	cpuId := 100 - cpuUs - cpuSy
	memTotal := 8192
	memUsed := 3000 + s.rng.Intn(2000)
	memFree := memTotal - memUsed

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("top - %s up 45 days, %2d users,  load average: %s\r\n",
		time.Now().Format("15:04:05"), 1+s.rng.Intn(3), load))
	sb.WriteString(fmt.Sprintf("Tasks: %3d total,   %d running, %3d sleeping,   0 stopped,   0 zombie\r\n",
		120+s.rng.Intn(50), 1+s.rng.Intn(3), 118+s.rng.Intn(50)))
	sb.WriteString(fmt.Sprintf("%%Cpu(s): %4.1f us, %4.1f sy,  0.0 ni, %4.1f id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st\r\n",
		cpuUs, cpuSy, cpuId))
	sb.WriteString(fmt.Sprintf("MiB Mem :   %d total,   %4d free,   %4d used,   %4d buff/cache\r\n",
		memTotal, memFree, memUsed, 1000+s.rng.Intn(1000)))
	sb.WriteString(fmt.Sprintf("MiB Swap:   2048 total,   2048 free,      0 used.   %4d avail Mem\r\n\r\n", memFree))
	sb.WriteString("  PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND\r\n")
	sb.WriteString(fmt.Sprintf(" %4d mysql     20   0 1234567 234567  12345 S  %4.1f  %4.1f   %s mysqld\r\n",
		500+s.rng.Intn(1000), 5.0+s.rng.Float64()*3, 15.0+s.rng.Float64()*3, s.randomDuration()))
	sb.WriteString(fmt.Sprintf(" %4d www-data  20   0  123456  45678   6789 S  %4.1f  %4.1f   %s nginx\r\n",
		500+s.rng.Intn(1000), 2.0+s.rng.Float64()*2, 2.5+s.rng.Float64()*2, s.randomDuration()))
	sb.WriteString(fmt.Sprintf(" %4d root      20   0   98765  12345   4567 S  %4.1f  %4.1f   %s systemd\r\n",
		1, 0.1+s.rng.Float64(), 0.5+s.rng.Float64(), s.randomDuration()))
	return sb.String()
}

func (s *SimShell) cmdFree() string {
	total := 8192 * 1024
	used := (3000 + s.rng.Intn(2000)) * 1024
	free := total - used
	buffCache := (1000 + s.rng.Intn(1000)) * 1024
	available := free + buffCache

	var sb strings.Builder
	sb.WriteString("              total        used        free      shared  buff/cache   available\r\n")
	sb.WriteString(fmt.Sprintf("Mem:       %8d    %8d    %8d    %8d    %8d    %8d\r\n",
		total, used, free, 500*1024, buffCache, available))
	sb.WriteString(fmt.Sprintf("Swap:      %8d    %8d    %8d\r\n", 2048*1024, 0, 2048*1024))
	return sb.String()
}

func (s *SimShell) cmdDF() string {
	disks := []struct {
		fs     string
		size   string
		used   string
		avail  string
		pct    int
		mount  string
	}{
		{"/dev/sda1", "50G", "18G", "30G", 38 + s.rng.Intn(10), "/"},
		{"/dev/sdb1", "100G", "45G", "50G", 45 + s.rng.Intn(15), "/data"},
		{"tmpfs", "4.0G", "0", "4.0G", 0, "/dev/shm"},
	}

	var sb strings.Builder
	sb.WriteString("Filesystem     Size  Used Avail Use% Mounted on\r\n")
	for _, d := range disks {
		sb.WriteString(fmt.Sprintf("%-14s %5s %5s %5s %3d%% %s\r\n", d.fs, d.size, d.used, d.avail, d.pct, d.mount))
	}
	return sb.String()
}

func (s *SimShell) cmdSystemctl(args []string) string {
	if len(args) < 2 || args[0] != "status" {
		return "演示模式：仅支持 systemctl status <service>\r\n"
	}

	service := args[1]
	services := map[string]struct {
		desc   string
		active string
		pid    int
		mem    string
	}{
		"nginx":  {"nginx - high performance web server", "active (running)", 800 + s.rng.Intn(100), "12.5M"},
		"mysql":  {"MySQL Community Server", "active (running)", 900 + s.rng.Intn(100), "456.2M"},
		"redis":  {"Redis In-Memory Data Store", "active (running)", 700 + s.rng.Intn(100), "8.3M"},
		"sshd":   {"OpenSSH server daemon", "active (running)", 600 + s.rng.Intn(100), "3.1M"},
		"docker": {"Docker Application Container Engine", "active (running)", 1000 + s.rng.Intn(100), "89.4M"},
	}

	svc, ok := services[service]
	if !ok {
		return fmt.Sprintf("Unit %s.service could not be found.\r\n", service)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("● %s.service - %s\r\n", service, svc.desc))
	sb.WriteString(fmt.Sprintf("   Loaded: loaded (/lib/systemd/system/%s.service; enabled; vendor preset: enabled)\r\n", service))
	sb.WriteString(fmt.Sprintf("   Active: %s since %s; 45 days ago\r\n", svc.active, time.Now().AddDate(0, 0, -45).Format("Mon 2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf(" Main PID: %d (%s)\r\n", svc.pid, service))
	sb.WriteString(fmt.Sprintf("   Memory: %s\r\n", svc.mem))
	sb.WriteString(fmt.Sprintf("   CGroup: /system.slice/%s.service\r\n", service))
	sb.WriteString(fmt.Sprintf("           └─%d /usr/sbin/%s\r\n", svc.pid, service))
	return sb.String()
}

func (s *SimShell) cmdTail(args []string) string {
	if len(args) == 0 {
		return "tail: missing file operand\r\n"
	}

	file := args[len(args)-1]
	logs := map[string][]string{
		"/var/log/syslog": {
			fmt.Sprintf("%s %s kernel: [12345.678901] eth0: link up, 1000Mbps, full-duplex", s.randomLogTime(), s.Hostname),
			fmt.Sprintf("%s %s systemd[1]: Started Daily apt upgrade and clean activities.", s.randomLogTime(), s.Hostname),
			fmt.Sprintf("%s %s sshd[1234]: Accepted password for root from 192.168.1.100 port 51234 ssh2", s.randomLogTime(), s.Hostname),
			fmt.Sprintf("%s %s CRON[5678]: (root) CMD (  cd / && run-parts --report /etc/cron.hourly)", s.randomLogTime(), s.Hostname),
			fmt.Sprintf("%s %s kernel: [23456.789012] TCP: eth0: Driver has suspect GRO implementation", s.randomLogTime(), s.Hostname),
		},
		"/var/log/nginx/access.log": {
			fmt.Sprintf(`192.168.1.100 - - [%s] "GET /api/v1/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, s.randomLogTime()),
			fmt.Sprintf(`192.168.1.101 - - [%s] "POST /api/v1/login HTTP/1.1" 200 567 "-" "Mozilla/5.0"`, s.randomLogTime()),
			fmt.Sprintf(`192.168.1.102 - - [%s] "GET /static/js/app.js HTTP/1.1" 304 0 "-" "Mozilla/5.0"`, s.randomLogTime()),
			fmt.Sprintf(`192.168.1.103 - - [%s] "GET /favicon.ico HTTP/1.1" 404 153 "-" "Mozilla/5.0"`, s.randomLogTime()),
			fmt.Sprintf(`192.168.1.104 - - [%s] "GET / HTTP/1.1" 200 5678 "-" "Mozilla/5.0"`, s.randomLogTime()),
		},
	}

	lines, ok := logs[file]
	if !ok {
		return fmt.Sprintf("tail: cannot open '%s' for reading: No such file or directory\r\n", file)
	}

	return strings.Join(lines, "\r\n") + "\r\n"
}

func (s *SimShell) cmdCat(args []string) string {
	if len(args) == 0 {
		return "cat: missing file operand\r\n"
	}

	file := args[0]
	files := map[string]string{
		"/etc/hostname":   s.Hostname + "\r\n",
		"/etc/os-release": "NAME=\"Ubuntu\"\r\nVERSION=\"22.04.3 LTS (Jammy Jellyfish)\"\r\nID=ubuntu\r\nID_LIKE=debian\r\nPRETTY_NAME=\"Ubuntu 22.04.3 LTS\"\r\nVERSION_ID=\"22.04\"\r\n",
		"/etc/hosts":      "127.0.0.1 localhost\r\n127.0.1.1 " + s.Hostname + "\r\n\r\n# The following lines are desirable for IPv6 capable hosts\r\n::1     ip6-localhost ip6-loopback\r\n",
	}

	content, ok := files[file]
	if !ok {
		return fmt.Sprintf("cat: %s: No such file or directory\r\n", file)
	}
	return content
}

// ========== 辅助函数 ==========

type fileInfo struct {
	Name  string
	Perm  string
	Links int
	Owner string
	Group string
	Size  int
}

func (s *SimShell) getFiles(path string) []fileInfo {
	if path == "." {
		path = s.CWD
	} else if !strings.HasPrefix(path, "/") {
		if s.CWD == "/" {
			path = "/" + path
		} else {
			path = s.CWD + "/" + path
		}
	}

	fs := map[string][]fileInfo{
		"/root": {
			{".bashrc", "-rw-------", 1, "root", "root", 3106},
			{".profile", "-rw-------", 1, "root", "root", 161},
			{"scripts", "drwxr-xr-x", 2, "root", "root", 4096},
			{"data", "drwxr-xr-x", 3, "root", "root", 4096},
			{"backup.tar.gz", "-rw-r--r--", 1, "root", "root", 1048576},
		},
		"/etc": {
			{"hostname", "-rw-r--r--", 1, "root", "root", 12},
			{"hosts", "-rw-r--r--", 1, "root", "root", 234},
			{"nginx", "drwxr-xr-x", 3, "root", "root", 4096},
			{"mysql", "drwxr-xr-x", 2, "root", "root", 4096},
			{"ssh", "drwxr-xr-x", 2, "root", "root", 4096},
		},
		"/var/log": {
			{"syslog", "-rw-r-----", 1, "syslog", "adm", 1048576},
			{"nginx", "drwxr-xr-x", 2, "root", "adm", 4096},
			{"mysql", "drwxr-xr-x", 2, "mysql", "adm", 4096},
		},
		"/": {
			{"bin", "drwxr-xr-x", 2, "root", "root", 4096},
			{"etc", "drwxr-xr-x", 5, "root", "root", 4096},
			{"home", "drwxr-xr-x", 3, "root", "root", 4096},
			{"root", "drwx------", 3, "root", "root", 4096},
			{"usr", "drwxr-xr-x", 10, "root", "root", 4096},
			{"var", "drwxr-xr-x", 8, "root", "root", 4096},
		},
	}

	files, ok := fs[path]
	if !ok {
		return nil
	}
	return files
}

func (s *SimShell) dirExists(path string) bool {
	dirs := []string{"/", "/root", "/etc", "/var", "/var/log", "/usr", "/home", "/root/scripts", "/root/data"}
	for _, d := range dirs {
		if path == d {
			return true
		}
	}
	return false
}

func (s *SimShell) randomDate() string {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	month := months[s.rng.Intn(12)]
	day := 1 + s.rng.Intn(28)
	hour := s.rng.Intn(24)
	min := s.rng.Intn(60)
	return fmt.Sprintf("%s %2d %02d:%02d", month, day, hour, min)
}

func (s *SimShell) randomTime() string {
	hour := s.rng.Intn(24)
	min := s.rng.Intn(60)
	return fmt.Sprintf("%02d:%02d", hour, min)
}

func (s *SimShell) randomDuration() string {
	hours := s.rng.Intn(100)
	mins := s.rng.Intn(60)
	secs := s.rng.Intn(60)
	return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
}

func (s *SimShell) randomLogTime() string {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	month := months[s.rng.Intn(12)]
	day := 1 + s.rng.Intn(28)
	hour := s.rng.Intn(24)
	min := s.rng.Intn(60)
	sec := s.rng.Intn(60)
	return fmt.Sprintf("%s %2d %02d:%02d:%02d", month, day, hour, min, sec)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
