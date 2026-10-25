﻿﻿﻿﻿﻿﻿﻿# ============================================================
# 平台管理工具
# 功能：系统服务 启停/重启、运行状态、数据备份/恢复（自定义备份目录）
# 适用：Windows 10 / Server 2016 及以上，PowerShell 5.1+
# 说明：软件名称、服务名、公司名、网址均从主程序动态获取，本脚本不含明文
# ============================================================

# --- 自动提权（服务控制需要管理员） ---
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$PSCommandPath`"" -Verb RunAs
    exit
}

Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()

# --- 从主程序动态获取品牌信息（key=value，UTF-8），避免脚本中出现明文 ---
function Get-About($exePath) {
    $h = @{ app = ''; service = 'QinJiaGateway'; company = ''; website = '' }
    if (-not (Test-Path $exePath)) { return $h }
    try {
        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = $exePath
        $psi.Arguments = '-about 1'
        $psi.RedirectStandardOutput = $true
        $psi.UseShellExecute = $false
        $psi.CreateNoWindow = $true
        $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
        $p = [System.Diagnostics.Process]::Start($psi)
        $out = $p.StandardOutput.ReadToEnd()
        $p.WaitForExit(15000) | Out-Null
        foreach ($line in ($out -split "`r?`n")) {
            if ($line -match '^([a-z]+)=(.*)$') { $h[$matches[1]] = $matches[2] }
        }
    } catch { }
    return $h
}

# --- 从 config.yaml 解析数据目录（安装时用户自定义路径以此为准） ---
# 不依赖 YAML 模块，手工解析 data_dir 一行并还原双引号转义
function Get-DataDirFromConfig($cfgPath) {
    if (-not (Test-Path $cfgPath)) { return '' }
    foreach ($line in Get-Content $cfgPath -Encoding UTF8 -ErrorAction SilentlyContinue) {
        if ($line -match '^\s*data_dir\s*:\s*(.+?)\s*$') {
            $v = $matches[1]
            if ($v -match '^"(.*)"$') {
                # YAML 双引号字符串：先还原 \\ 再还原 \"（路径中不含其他转义）
                $v = $matches[1] -replace '\\\\', '\' -replace '\\"', '"'
            } elseif ($v -match "^'(.*)'$") {
                $v = $matches[1]
            }
            return $v
        }
    }
    return ''
}

# --- 路径解析（兼容安装版与解压版） ---
$scriptDir  = Split-Path $PSCommandPath -Parent
$exeFile    = Join-Path $scriptDir 'gateway.exe'
$about      = Get-About $exeFile
$SERVICE    = $about.service
$APPNAME    = if ($about.app) { $about.app } else { '运维管理平台' }

$cfgFile    = Join-Path $scriptDir 'config.yaml'
$cfgDataDir = Get-DataDirFromConfig $cfgFile
if ($cfgDataDir) {
    # 相对路径（如解压版 ./data）相对于 config.yaml 所在目录解析
    if ([System.IO.Path]::IsPathRooted($cfgDataDir)) {
        $dataDir = $cfgDataDir
    } else {
        $dataDir = [System.IO.Path]::GetFullPath((Join-Path $scriptDir $cfgDataDir))
    }
} elseif (Test-Path 'C:\ProgramData\QinJiaGateway\data\gateway.db') {
    # 兼容旧版本（config.yaml 无 data_dir 时）
    $dataDir = 'C:\ProgramData\QinJiaGateway\data'
} else {
    $dataDir = Join-Path $scriptDir 'data'
}
# manager.conf 位置独立于数据目录：公共数据目录存在则用之，否则脚本目录
if (Test-Path 'C:\ProgramData\QinJiaGateway') {
    $baseDir = 'C:\ProgramData\QinJiaGateway'
} else {
    $baseDir = $scriptDir
}
$dbFile   = Join-Path $dataDir 'gateway.db'
$confFile = Join-Path $baseDir 'manager.conf'

# 默认备份目录：用户「文档」下，明确独立于程序安装路径
$defaultBackupDir = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'QinJiaGateway-Backups'

# --- 备份目录设置持久化（key=value 简单文本） ---
function Get-Conf {
    $h = @{}
    if (Test-Path $confFile) {
        foreach ($line in Get-Content $confFile -ErrorAction SilentlyContinue) {
            if ($line -match '^\s*([^#=]+?)\s*=\s*(.*)$') { $h[$matches[1].Trim()] = $matches[2].Trim() }
        }
    }
    return $h
}
function Save-Conf($dir) {
    New-Item -ItemType Directory -Path $baseDir -Force | Out-Null
    "BackupDir=$dir" | Set-Content -Path $confFile -Encoding UTF8
}

# --- 目录归属判断（忽略大小写，处理尾部反斜杠） ---
function Test-Inside($child, $parent) {
    try {
        $c = [System.IO.Path]::GetFullPath($child).TrimEnd('\')
        $p = [System.IO.Path]::GetFullPath($parent).TrimEnd('\')
    } catch { return $false }
    if ($c -ieq $p) { return $true }
    return $c.StartsWith($p + '\', [System.StringComparison]::OrdinalIgnoreCase)
}

# --- 校验并准备备份目录，失败抛异常 ---
function Resolve-BackupDir($raw) {
    if ([string]::IsNullOrWhiteSpace($raw)) {
        throw '备份目录不能为空。'
    }
    $full = [System.IO.Path]::GetFullPath($raw.Trim()).TrimEnd('\')
    if (Test-Inside $full $scriptDir) {
        throw "备份目录不能位于程序安装路径下：`n$scriptDir`n`n请选择其他位置（如其他磁盘、「文档」目录或网盘同步目录）。"
    }
    if (Test-Inside $full $dataDir) {
        throw "备份目录不能位于平台数据目录下：`n$dataDir`n`n请选择其他位置。"
    }
    if (-not (Test-Path $full)) {
        New-Item -ItemType Directory -Path $full -Force -ErrorAction Stop | Out-Null
    }
    # 可写性探测
    $probe = Join-Path $full ('.qj-write-test-' + [guid]::NewGuid().ToString('N'))
    try {
        [System.IO.File]::WriteAllText($probe, 't')
        Remove-Item $probe -Force
    } catch {
        throw "所选目录不可写，请更换目录：`n$full"
    }
    return $full
}

# 初始备份目录：优先上次保存值，异常则回退默认值（不强制创建，备份时再建）
$backupDir = $defaultBackupDir
$saved = (Get-Conf)['BackupDir']
if ($saved) {
    try { $backupDir = Resolve-BackupDir $saved } catch { $backupDir = $defaultBackupDir }
}

# --- 服务操作 ---
function Get-SvcStatus {
    $s = Get-Service -Name $SERVICE -ErrorAction SilentlyContinue
    if ($s) { return $s.Status.ToString() } else { return '未注册' }
}
function Start-Svc {
    # 不再吞掉 sc.exe 输出：启动失败时弹窗告知原因与日志位置
    $out = (& sc.exe start $SERVICE 2>&1) -join "`n"
    Start-Sleep -Seconds 2
    if ((Get-SvcStatus) -ne 'Running') {
        $logHint = Join-Path $dataDir 'logs'
        [System.Windows.Forms.MessageBox]::Show(
            "服务启动失败或已退出。`n`n$out`n`n详细原因请查看日志目录：`n$logHint`n（gateway-日期.log）`n`n常见原因：端口被占用、数据目录权限不足。",
            '启动失败', 'OK', 'Error') | Out-Null
    }
}
function Stop-Svc  { & sc.exe stop  $SERVICE | Out-Null }
function Refresh-Status {
    $st = Get-SvcStatus
    $lblStatus.Text = "服务状态：$st"
    if ($st -eq 'Running') {
        $lblStatus.ForeColor = [System.Drawing.Color]::FromArgb(0x2e,0xa0,0x43)
        # 运行中：启动禁用（灰色），停止/重启可用
        $btnStart.Enabled   = $false
        $btnStop.Enabled    = $true
        $btnRestart.Enabled = $true
    } elseif ($st -eq 'Stopped') {
        $lblStatus.ForeColor = [System.Drawing.Color]::FromArgb(0xe3,0x7b,0x2e)
        # 已停止：停止/重启禁用（灰色），启动可用
        $btnStart.Enabled   = $true
        $btnStop.Enabled    = $false
        $btnRestart.Enabled = $false
    } else {
        # 未注册 或 正在启动/停止等过渡状态：服务操作按钮全部禁用，防重复点击
        $lblStatus.ForeColor = [System.Drawing.Color]::FromArgb(0x88,0x92,0xa0)
        $btnStart.Enabled   = $false
        $btnStop.Enabled    = $false
        $btnRestart.Enabled = $false
    }
}
function Refresh-BackupList {
    $cboBackups.Items.Clear()
    Get-ChildItem $script:backupDir -Filter 'gateway-*.db' -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | ForEach-Object { [void]$cboBackups.Items.Add($_.Name) }
    if ($cboBackups.Items.Count -gt 0) { $cboBackups.SelectedIndex = 0 }
}

# 校验文本框中的目录并应用（保存设置 + 刷新列表）
function Apply-BackupDir($raw) {
    try {
        $resolved = Resolve-BackupDir $raw
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, '备份目录无效', 'OK', 'Error') | Out-Null
        return $false
    }
    $script:backupDir = $resolved
    $txtBakDir.Text = $resolved
    Save-Conf $resolved
    Refresh-BackupList
    return $true
}

# --- 数据备份（服务运行时先停止，完成后恢复原状态） ---
function Do-Backup {
    if (-not (Apply-BackupDir $txtBakDir.Text.Trim())) { return }
    if (-not (Test-Path $dbFile)) {
        [System.Windows.Forms.MessageBox]::Show('未找到数据库文件，无法备份。','提示') | Out-Null; return
    }
    $wasRunning = (Get-SvcStatus) -eq 'Running'
    if ($wasRunning) { Stop-Svc; Start-Sleep -Seconds 2 }
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $dst = Join-Path $script:backupDir "gateway-$stamp.db"
    Copy-Item $dbFile $dst -Force
    if ($wasRunning) { Start-Svc }
    Refresh-BackupList
    Refresh-Status
    [System.Windows.Forms.MessageBox]::Show("备份完成：`n$dst",'提示') | Out-Null
}

# --- 数据恢复 ---
function Do-Restore {
    if (-not (Apply-BackupDir $txtBakDir.Text.Trim())) { return }
    if (-not $cboBackups.SelectedItem) {
        [System.Windows.Forms.MessageBox]::Show('请先选择要恢复的备份文件。','提示') | Out-Null; return
    }
    $src = Join-Path $script:backupDir $cboBackups.SelectedItem
    $r = [System.Windows.Forms.MessageBox]::Show(
        "确定用以下备份覆盖当前数据吗？当前数据将被替换：`n$($cboBackups.SelectedItem)",
        '恢复确认','YesNo','Warning')
    if ($r -ne 'Yes') { return }
    $wasRunning = (Get-SvcStatus) -eq 'Running'
    if ($wasRunning) { Stop-Svc; Start-Sleep -Seconds 2 }
    Copy-Item $src $dbFile -Force
    if ($wasRunning) { Start-Svc }
    Refresh-Status
    [System.Windows.Forms.MessageBox]::Show('数据恢复完成。','提示') | Out-Null
}

# ================= 授权管理 =================
# --- 调用 gateway.exe -licinfo 获取授权状态（UTF-8 key=value），
#     数据目录存在 license.lic 时子进程会顺带激活并回传具体错误 ---
function Get-LicInfo {
    $h = [ordered]@{ edition=''; max_servers=''; expire_at=''; issued_to='';
                    is_trial='false'; days_left=''; servers=''; activate_error='' }
    try {
        $psi = New-Object System.Diagnostics.ProcessStartInfo
        $psi.FileName = $exeFile
        $psi.Arguments = '-licinfo 1'
        $psi.RedirectStandardOutput = $true
        $psi.UseShellExecute = $false
        $psi.CreateNoWindow = $true
        $psi.WorkingDirectory = $scriptDir
        $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
        $p = [System.Diagnostics.Process]::Start($psi)
        $out = $p.StandardOutput.ReadToEnd()
        $p.WaitForExit(30000) | Out-Null
        foreach ($line in ($out -split "`r?`n")) {
            if ($line -match '^([a-z_]+)=(.*)$' -and $h.Contains($matches[1])) {
                $h[$matches[1]] = $matches[2]
            }
        }
    } catch { }
    return $h
}

# --- 刷新授权状态显示 ---
$greenColor = [System.Drawing.Color]::FromArgb(0x2e,0xa0,0x43)
$orangeColor = [System.Drawing.Color]::FromArgb(0xe3,0x7b,0x2e)
$darkColor = [System.Drawing.Color]::FromArgb(0x2b,0x2f,0x36)
function Refresh-License {
    $li = Get-LicInfo
    if ($li.edition -eq 'enterprise') {
        if ($li.is_trial -eq 'true') {
            $lblLicVer.Text = '当前授权：企业版试用（不限服务器数量）'
            $lblLicVer.ForeColor = $orangeColor
        } else {
            $lblLicVer.Text = '当前授权：企业版正式授权（不限服务器数量）'
            $lblLicVer.ForeColor = $greenColor
        }
    } else {
        $lblLicVer.Text = '当前授权：社区版（无服务器数量与使用期限限制）'
        $lblLicVer.ForeColor = $greenColor
    }
    if ($li.days_left -eq '-1') {
        $lblLicExpire.Text = '有效期：永久授权，无使用期限限制'
        $lblLicExpire.ForeColor = $darkColor
    } else {
        $lblLicExpire.Text = "有效期：至 $($li.expire_at)（剩余 $($li.days_left) 天）"
        if ([int]$li.days_left -le 15) { $lblLicExpire.ForeColor = $orangeColor }
        else { $lblLicExpire.ForeColor = $darkColor }
    }
    if ($li.issued_to) {
        $lblLicTo.Text = "授权对象：$($li.issued_to)"
    } else {
        $lblLicTo.Text = '授权对象：—（社区版）'
    }
    $lblLicTo.ForeColor = $darkColor
    return $li
}

# --- 导入授权文件（激活走 -licinfo 子进程，服务停止状态下同样可用） ---
function Do-ImportLicense {
    $dlg = New-Object System.Windows.Forms.OpenFileDialog
    $dlg.Title = '选择授权文件（.lic / .txt）'
    $dlg.Filter = '授权文件 (*.lic;*.txt)|*.lic;*.txt|所有文件 (*.*)|*.*'
    $dlg.CheckFileExists = $true
    if ($dlg.ShowDialog() -ne 'OK') { $dlg.Dispose(); return }
    $src = $dlg.FileName
    $dlg.Dispose()

    $wasRunning = (Get-SvcStatus) -eq 'Running'
    $stopText = if ($wasRunning) { '临时停止' } else { '无需停止' }
    $r = [System.Windows.Forms.MessageBox]::Show(
        "导入授权将$stopText服务，激活后自动恢复原状态。`n`n授权文件：$src`n`n是否继续？",
        '导入授权', 'YesNo', 'Question')
    if ($r -ne 'Yes') { return }

    $licDst = Join-Path $dataDir 'license.lic'
    $failedDst = "$licDst.failed"
    try {
        if (-not (Test-Path $dataDir)) {
            New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
        }
        if ($wasRunning) { Stop-Svc; Start-Sleep -Seconds 2 }
        # 清理旧目标后按原始字节复制（后端会完整校验，含文件编码/BOM处理）
        Remove-Item $licDst, $failedDst -Force -ErrorAction SilentlyContinue
        Copy-Item $src $licDst -Force

        # -licinfo 子进程执行激活，返回精确错误（不依赖服务运行）
        $li = Get-LicInfo
        if ($li.activate_error) { throw $li.activate_error }

        Remove-Item $failedDst -Force -ErrorAction SilentlyContinue
        Refresh-License

        if ($wasRunning) {
            Start-Svc; Start-Sleep -Seconds 2
            Refresh-Status
            [System.Windows.Forms.MessageBox]::Show(
                "授权激活成功，服务已恢复运行。`n`n$($lblLicVer.Text)`n$($lblLicExpire.Text)",
                '授权管理', 'OK', 'Information') | Out-Null
        } else {
            $ask = [System.Windows.Forms.MessageBox]::Show(
                "授权激活成功。`n`n$($lblLicVer.Text)`n$($lblLicExpire.Text)`n`n是否立即启动服务？",
                '授权管理', 'YesNo', 'Question')
            if ($ask -eq 'Yes') {
                Start-Svc; Start-Sleep -Seconds 2
                Refresh-Status
            }
        }
    } catch {
        if ($wasRunning -and (Get-SvcStatus) -ne 'Running') {
            Start-Svc; Start-Sleep -Seconds 1
        }
        Refresh-Status
        [System.Windows.Forms.MessageBox]::Show(
            "授权激活失败：`n`n$($_.Exception.Message)`n`n授权文件已保留在：`n$licDst`n请核对后重新导入。",
            '授权管理', 'OK', 'Error') | Out-Null
    }
}

# ================= GUI =================
$form = New-Object System.Windows.Forms.Form
$form.Text = "$APPNAME - 管理工具"
$form.Size = New-Object System.Drawing.Size(680, 625)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.BackColor = [System.Drawing.Color]::FromArgb(245,247,250)
$form.Font = New-Object System.Drawing.Font('Microsoft YaHei UI', 9)

$blue = [System.Drawing.Color]::FromArgb(47,129,247)

# 标题
$lblTitle = New-Object System.Windows.Forms.Label
$lblTitle.Text = '平台管理工具'
$lblTitle.Font = New-Object System.Drawing.Font('Microsoft YaHei UI', 15, [System.Drawing.FontStyle]::Bold)
$lblTitle.ForeColor = $blue
$lblTitle.AutoSize = $true
$lblTitle.Location = New-Object System.Drawing.Point(24, 16)
$form.Controls.Add($lblTitle)

# 企业 logo（靠右显示，与标题垂直居中）
$picLogo = New-Object System.Windows.Forms.PictureBox
$logoPath = Join-Path $scriptDir 'qinplus.png'
if (Test-Path $logoPath) {
    $picLogo.Image = [System.Drawing.Image]::FromFile($logoPath)
    $picLogo.SizeMode = [System.Windows.Forms.PictureBoxSizeMode]::Zoom
    $picLogo.BackColor = [System.Drawing.Color]::Transparent
    $picLogo.Size = New-Object System.Drawing.Size(134, 26)
    $logoX = $form.ClientSize.Width - 24 - 134
    $logoY = 16 + [int](($lblTitle.PreferredHeight - 26) / 2)
    $picLogo.Location = New-Object System.Drawing.Point($logoX, $logoY)
} else {
    $picLogo.Visible = $false
}
$form.Controls.Add($picLogo)

# ---- 分组1：服务管理 ----
$grpSvc = New-Object System.Windows.Forms.GroupBox
$grpSvc.Text = '系统服务管理'
$grpSvc.Size = New-Object System.Drawing.Size(616, 120)
$grpSvc.Location = New-Object System.Drawing.Point(24, 56)
$form.Controls.Add($grpSvc)

$lblStatus = New-Object System.Windows.Forms.Label
$lblStatus.AutoSize = $true
$lblStatus.Font = New-Object System.Drawing.Font('Microsoft YaHei UI', 10.5, [System.Drawing.FontStyle]::Bold)
$lblStatus.Location = New-Object System.Drawing.Point(18, 26)
$grpSvc.Controls.Add($lblStatus)

function Make-Button($text,$x,$y,$w,$h) {
    $b = New-Object System.Windows.Forms.Button
    $b.Text = $text; $b.Size = New-Object System.Drawing.Size($w,$h)
    $b.Location = New-Object System.Drawing.Point($x,$y)
    $b.FlatStyle = 'Flat'; $b.FlatAppearance.BorderColor = $blue
    $b.BackColor = [System.Drawing.Color]::White; $b.Cursor = 'Hand'
    return $b
}
$btnStart   = Make-Button '启动服务' 18  62 120 34
$btnStop    = Make-Button '停止服务' 162 62 120 34
$btnRestart = Make-Button '重启服务' 306 62 120 34
$btnRefresh = Make-Button '刷新状态' 450 62 120 34
$btnStart.Add_Click({ Start-Svc; Start-Sleep -Seconds 1; Refresh-Status })
$btnStop.Add_Click({ Stop-Svc; Start-Sleep -Seconds 1; Refresh-Status })
$btnRestart.Add_Click({ Stop-Svc; Start-Sleep -Seconds 2; Start-Svc; Start-Sleep -Seconds 1; Refresh-Status })
$btnRefresh.Add_Click({ Refresh-Status })
$grpSvc.Controls.AddRange(@($btnStart,$btnStop,$btnRestart,$btnRefresh))

# ---- 分组2：授权管理 ----
$grpLic = New-Object System.Windows.Forms.GroupBox
$grpLic.Text = '授权管理'
$grpLic.Size = New-Object System.Drawing.Size(616, 96)
$grpLic.Location = New-Object System.Drawing.Point(24, 190)
$form.Controls.Add($grpLic)

function Make-InfoLabel($x,$y,$w) {
    $l = New-Object System.Windows.Forms.Label
    $l.Size = New-Object System.Drawing.Size($w, 20)
    $l.Location = New-Object System.Drawing.Point($x, $y)
    $l.AutoSize = $false
    $l.AutoEllipsis = $true
    return $l
}
$lblLicVer    = Make-InfoLabel 18  23 430
$lblLicVer.Font = New-Object System.Drawing.Font('Microsoft YaHei UI', 10, [System.Drawing.FontStyle]::Bold)
$lblLicExpire = Make-InfoLabel 18  47 430
$lblLicTo     = Make-InfoLabel 18  69 430
$grpLic.Controls.AddRange(@($lblLicVer,$lblLicExpire,$lblLicTo))

# 导入授权文件
$btnImportLic = Make-Button '导入授权文件...' 463 24 135 33
$btnImportLic.BackColor = $blue; $btnImportLic.ForeColor = [System.Drawing.Color]::White
$btnImportLic.Add_Click({ Do-ImportLicense })
$grpLic.Controls.Add($btnImportLic)

# ---- 分组3：数据备份 ----
$grpBak = New-Object System.Windows.Forms.GroupBox
$grpBak.Text = '数据备份与恢复'
$grpBak.Size = New-Object System.Drawing.Size(616, 195)
$grpBak.Location = New-Object System.Drawing.Point(24, 300)
$form.Controls.Add($grpBak)

$btnBackup = Make-Button '立即备份' 18 28 100 36
$btnBackup.BackColor = $blue; $btnBackup.ForeColor = [System.Drawing.Color]::White
$btnBackup.Add_Click({ Do-Backup })
$grpBak.Controls.Add($btnBackup)

# 备份位置（可直接粘贴路径，或点浏览选择）
$lblPos = New-Object System.Windows.Forms.Label
$lblPos.Text = '备份位置：'
$lblPos.AutoSize = $true
$lblPos.Location = New-Object System.Drawing.Point(18, 79)
$grpBak.Controls.Add($lblPos)

$txtBakDir = New-Object System.Windows.Forms.TextBox
$txtBakDir.Size = New-Object System.Drawing.Size(380, 26)
$txtBakDir.Location = New-Object System.Drawing.Point(96, 75)
$grpBak.Controls.Add($txtBakDir)

$btnBrowse = Make-Button '浏览...' 484 73 83 30
$btnBrowse.Add_Click({
    $dlg = New-Object System.Windows.Forms.FolderBrowserDialog
    $dlg.Description = '选择数据备份目录（不可选择程序安装路径）'
    $dlg.ShowNewFolderButton = $true
    if (Test-Path $script:backupDir) { $dlg.SelectedPath = $script:backupDir }
    if ($dlg.ShowDialog() -eq 'OK') { [void](Apply-BackupDir $dlg.SelectedPath) }
    $dlg.Dispose()
})
$grpBak.Controls.Add($btnBrowse)

$lblBak = New-Object System.Windows.Forms.Label
$lblBak.Text = '选择备份：'
$lblBak.AutoSize = $true
$lblBak.Location = New-Object System.Drawing.Point(18, 123)
$grpBak.Controls.Add($lblBak)

$cboBackups = New-Object System.Windows.Forms.ComboBox
$cboBackups.DropDownStyle = 'DropDownList'
$cboBackups.Size = New-Object System.Drawing.Size(380, 26)
$cboBackups.Location = New-Object System.Drawing.Point(96, 119)
$grpBak.Controls.Add($cboBackups)

$btnRestore = Make-Button '恢复所选' 484 117 83 30
$btnRestore.Add_Click({ Do-Restore })
$grpBak.Controls.Add($btnRestore)

$lblHint = New-Object System.Windows.Forms.Label
$lblHint.Text = '备份独立存放，建议选其他磁盘或网盘同步目录；卸载/重装平台不影响备份。'
$lblHint.AutoSize = $true
$lblHint.ForeColor = [System.Drawing.Color]::Gray
$lblHint.Location = New-Object System.Drawing.Point(18, 158)
$grpBak.Controls.Add($lblHint)

# ---- 快捷入口 ----
$btnWeb  = Make-Button '打开平台网页' 24  510 192 36
$btnData = Make-Button '打开数据目录' 232 510 192 36
$btnCfg  = Make-Button '查看配置文件' 440 510 192 36
$btnWeb.Add_Click({
    $port = (Select-String -Path (Join-Path $scriptDir 'config.yaml') -Pattern '^\s*port:\s*(\d+)').Matches.Groups[1].Value
    if (-not $port) { $port = '8080' }
    Start-Process "http://localhost:$port/"
})
$btnData.Add_Click({
    if (-not (Test-Path $dataDir)) {
        New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
    }
    Start-Process explorer.exe $dataDir
})
$btnCfg.Add_Click({
    $cfgPath = Join-Path $scriptDir 'config.yaml'
    if (Test-Path $cfgPath) { Start-Process notepad.exe $cfgPath }
})
$form.Controls.AddRange(@($btnWeb,$btnData,$btnCfg))

# 版权（公司名、网址来自主程序，无联系电话；自动水平居中）
$lblCopy = New-Object System.Windows.Forms.Label
$lblCopy.Text = "$($about.company)  $($about.website)"
$lblCopy.AutoSize = $true
$lblCopy.ForeColor = [System.Drawing.Color]::Gray
$form.Controls.Add($lblCopy)
$lblCopy.Location = New-Object System.Drawing.Point(
    [int](($form.ClientSize.Width - $lblCopy.PreferredWidth) / 2), 558)

# 初始化
$txtBakDir.Text = $backupDir
Refresh-Status
Refresh-License | Out-Null
Refresh-BackupList
[void]$form.ShowDialog()
