; ============================================================
; 勤加软件WEB服务器运维管理平台（演示版）- Inno Setup 安装脚本
; 演示版：内置模拟服务器，管理员账号锁定，无需真实服务器即可体验
; ============================================================
#define MyAppVersion "1.0.0"
#define DefaultAppName "勤加软件WEB服务器运维管理平台（演示版）"
#define DefaultSvcName "QinJiaGatewayDemo"

[Setup]
AppId={{QINJIA-DEMO2-2026-1001
AppName={code:GetAppName}
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\QinJiaGatewayDemo
DefaultGroupName={#DefaultAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=勤加软件WEB服务器运维管理平台-演示版
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
DisableWelcomePage=no
VersionInfoDescription={#DefaultAppName}安装程序
VersionInfoProductName={#DefaultAppName}
WizardImageFile=wizard-image-164.bmp,wizard-image-328.bmp
WizardSmallImageFile=wizard-small-55.bmp,wizard-small-110.bmp
SetupIconFile=app.ico
UninstallDisplayIcon={app}\gateway.exe
LicenseFile=用户许可协议.txt

[Languages]
Name: "chinesesimplified"; MessagesFile: "ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式（浏览器访问 + 服务管理）"; GroupDescription: "附加选项:"; Flags: checkedonce

[Files]
Source: "gateway.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "app.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "config-demo.yaml"; DestDir: "{app}"; DestName: "config.yaml"; Flags: ignoreversion onlyifdoesntexist
Source: "manager.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "manager.vbs"; DestDir: "{app}"; Flags: ignoreversion
Source: "qinplus.png"; DestDir: "{app}"; Flags: ignoreversion
Source: "用户许可协议.txt"; DestDir: "{app}"; Flags: ignoreversion
; 演示版无需授权工具（demo 模式跳过授权校验）

[Icons]
; 快捷方式说明：
;   URL 使用 {computername}（本机名）而非 localhost——把快捷方式发送/复制到局域网其他电脑后，
;   仍能解析到装了平台的服务器；本机访问时主机名同样解析到自己，行为一致。
;   图标引用随包安装的独立 app.ico（比从 exe 提取更稳定）。
; —— 开始菜单（{autoprograms} 随安装模式自动定位，名称随软件名称联动）——
Name: "{autoprograms}\{code:GetAppName}\{code:GetAppName}"; Filename: "http://{computername}:{code:GetPort}/"; IconFilename: "{app}\app.ico"; IconIndex: 0
Name: "{autoprograms}\{code:GetAppName}\{code:GetAppName}管理工具"; Filename: "{sys}\wscript.exe"; Parameters: """{app}\manager.vbs"""; IconFilename: "{app}\app.ico"; IconIndex: 0
Name: "{autoprograms}\{code:GetAppName}\卸载"; Filename: "{uninstallexe}"
; —— 桌面 ——
Name: "{autodesktop}\{code:GetAppName}"; Filename: "http://{computername}:{code:GetPort}/"; IconFilename: "{app}\app.ico"; IconIndex: 0; Tasks: desktopicon
Name: "{autodesktop}\{code:GetAppName}管理工具"; Filename: "{sys}\wscript.exe"; Parameters: """{app}\manager.vbs"""; IconFilename: "{app}\app.ico"; IconIndex: 0; Tasks: desktopicon

[UninstallDelete]
; config.yaml 为安装后动态生成，需显式删除；数据目录默认保留
Type: files; Name: "{app}\config.yaml"

[Code]
var
  NamePage, SettingsPage, PrevDirPage: TWizardPage;
  DataPage: TInputDirWizardPage;
  AppNameLbl, SvcLbl, PrevDirLbl, PrevDirTip: TNewStaticText;
  PrevDirEdit: TNewEdit;
  AppNameEdit, SvcEdit: TNewEdit;
  PortLbl, DemoAcctLbl, TipLbl: TNewStaticText;
  PortEdit: TNewEdit;
  AppNameValue, SvcValue, PortValue, DataDirValue, PrevDirValue: String;
  UpgradeMode: Boolean; // 已安装版本直接覆盖更新模式（/SILENT /UPGRADE=1，保留配置与数据）
  CompletionOpenWeb, CompletionOpenMgr: Boolean; // 安装完成提示窗体的勾选结果

// 仅将 ASCII 小写字母转为大写，其他字符（尤其中文）原样保留，
// 避免 Uppercase 在个别系统上改变含 CJK 字符串的长度而造成定位错位
function AsciiUpper(s: String): String;
var i: Integer; c: Char;
begin
  Result := s;
  for i := 1 to Length(Result) do begin
    c := Result[i];
    if (c >= 'a') and (c <= 'z') then Result[i] := Chr(Ord(c) - 32);
  end;
end;

// 读取安装命令行参数 /APPNAME= /SVC= /PORT= /ADMIN= /PASS=（用于静默部署）
// 注意：Pascal Script 的 Pos 对含 CJK 的字符串按 ANSI(GBK) 字节位置计数，
// 与 Length/Copy 的字符位置不一致，因此此处全程手工按字符扫描。
function GetCmdParam(name, def: String): String;
var tail, s: String; plen, i, j, cut: Integer; ok: Boolean;
begin
  Result := def;
  tail := GetCmdTail();
  name := AsciiUpper(name);
  plen := Length(name);

  // 手工扫描 "/name="（ASCII 大小写不敏感）
  for i := 1 to Length(tail) - (plen + 1) do begin
    if tail[i] <> '/' then Continue;
    ok := True;
    for j := 1 to plen do begin
      if AsciiUpper(tail[i + j]) <> name[j] then begin ok := False; Break; end;
    end;
    if not ok or (tail[i + plen + 1] <> '=') then Continue;

    s := Copy(tail, i + plen + 2, MaxInt);
    if Copy(s, 1, 1) = '"' then begin
      s := Copy(s, 2, MaxInt);
      for cut := 1 to Length(s) do begin
        if s[cut] = '"' then begin Result := Copy(s, 1, cut - 1); Exit; end;
      end;
      Result := s; Exit;
    end else begin
      for cut := 1 to Length(s) do begin
        if (s[cut] = ' ') or (Ord(s[cut]) = 9) then begin
          Result := Copy(s, 1, cut - 1); Exit;
        end;
      end;
      Result := s; Exit;
    end;
  end;
end;

// 读取已安装版本的安装目录：直接查询卸载注册表项的 InstallLocation 值。
// 不能用 GetPreviousData——它仅读取安装时通过 SetPreviousData 写入的自定义数据，
// 读不到 InstallLocation 这类标准值。
function ReadPrevInstallDir(): String;
var sub, key64, key32: String;
begin
  Result := '';
  // 真实卸载键名无右花括号（AppId 双花括号写法所致）：
  // 形如 {QINJIA-WEBGATEWAY-2026-1001_is1，末尾不要加 }
  sub := Chr(123) + 'QINJIA-DEMO2-2026-1001_is1';
  key64 := 'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\' + sub;
  key32 := 'SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\' + sub;
  if RegQueryStringValue(HKLM, key64, 'InstallLocation', Result) then Exit;
  if RegQueryStringValue(HKLM, key32, 'InstallLocation', Result) then Exit;
end;

// 从已安装版本的 config.yaml 中提取 data_dir 值（去掉 YAML 引号并反转义）
function ReadDataDirFromConfig(appDir: String): String;
var cfg, line, v: String; Lines: TArrayOfString; i: Integer;
begin
  Result := '';
  if appDir = '' then Exit;
  // 去掉目录结尾的反斜杠（保留盘符根目录）
  while (Length(appDir) > 3) and (appDir[Length(appDir)] = '\') do
    appDir := Copy(appDir, 1, Length(appDir) - 1);
  cfg := appDir + '\config.yaml';
  if not FileExists(cfg) then Exit;
  if not LoadStringsFromFile(cfg, Lines) then Exit;
  for i := 0 to GetArrayLength(Lines) - 1 do begin
    line := Trim(Lines[i]);
    if Pos('data_dir:', line) <> 1 then Continue;
    v := Trim(Copy(line, Pos(':', line) + 1, MaxInt));
    // 去掉 YAML 双引号或单引号
    if (Length(v) >= 2) and (v[1] = '"') and (v[Length(v)] = '"') then
      v := Copy(v, 2, Length(v) - 2)
    else if (Length(v) >= 2) and (v[1] = '''') and (v[Length(v)] = '''') then
      v := Copy(v, 2, Length(v) - 2);
    // YAML 双引号字符串反转义：\\ -> \, \" -> "
    StringChange(v, '\\', '\');
    StringChange(v, '\"', '"');
    Result := Trim(v);
    Exit;
  end;
end;

// 从已安装版本的 config.yaml 中提取任意顶层/二级键值（如 name/service_name/port）。
// 匹配规则：去掉行首空白后以 "key:" 开头，取冒号后的值并去 YAML 引号与转义。
function ReadCfgValue(appDir, key: String): String;
var cfg, line, v: String; Lines: TArrayOfString; i: Integer;
begin
  Result := '';
  if (appDir = '') or (key = '') then Exit;
  while (Length(appDir) > 3) and (appDir[Length(appDir)] = '\') do
    appDir := Copy(appDir, 1, Length(appDir) - 1);
  cfg := appDir + '\config.yaml';
  if not FileExists(cfg) then Exit;
  if not LoadStringsFromFile(cfg, Lines) then Exit;
  for i := 0 to GetArrayLength(Lines) - 1 do begin
    line := Trim(Lines[i]);
    if Pos(key + ':', line) <> 1 then Continue;
    v := Trim(Copy(line, Length(key) + 2, MaxInt));
    if (Length(v) >= 2) and (v[1] = '"') and (v[Length(v)] = '"') then
      v := Copy(v, 2, Length(v) - 2)
    else if (Length(v) >= 2) and (v[1] = '''') and (v[Length(v)] = '''') then
      v := Copy(v, 2, Length(v) - 2);
    StringChange(v, '\\', '\');
    StringChange(v, '\"', '"');
    Result := Trim(v);
    Exit;
  end;
end;

// 检测是否静默安装（/SILENT 或 /VERYSILENT）。
// 直接扫描命令行尾部，不依赖 IsSilent 支持函数；仅作布尔判定，
// 不使用 Pos 的位置值，因此 CJK 字节/字符计数差异无影响。
function IsSilentInstall(): Boolean;
var tail: String;
begin
  tail := AsciiUpper(GetCmdTail());
  Result := (Pos('/VERYSILENT', tail) > 0) or (Pos('/SILENT', tail) > 0);
end;

// 检测是否为极静安装（仅 /VERYSILENT）。用于区分是否弹出完成提示：
// /SILENT（含交互升级）仍弹出完成提示；/VERYSILENT（全自动部署）则跳过。
function IsVerySilentInstall(): Boolean;
var tail: String;
begin
  tail := AsciiUpper(GetCmdTail());
  Result := Pos('/VERYSILENT', tail) > 0;
end;

// 安装最早期预初始化（静默安装时 [Setup]/[Icons] 常量即依赖这些值）
function InitializeSetup(): Boolean;
var prevName, prevSvc, prevPort, prevCfgDataDir: String;
begin
  AppNameValue := GetCmdParam('APPNAME', '{#DefaultAppName}');
  SvcValue := GetCmdParam('SVC', '{#DefaultSvcName}');
  PortValue := GetCmdParam('PORT', '8080');
  DataDirValue := GetCmdParam('DATADIR', '');
  PrevDirValue := ReadPrevInstallDir();

  // 检测到已安装：交互式弹窗询问是否更新；静默安装自动进入升级模式。
  // 确认/自动升级后在同一进程内进入升级模式（跳过所有向导页，直接显示安装进度），
  // 配置文件与业务数据原样保留，安装完成后弹出"打开Web/管理工具"提示
  if PrevDirValue <> '' then begin
    prevName := ReadCfgValue(PrevDirValue, 'name');
    prevSvc := ReadCfgValue(PrevDirValue, 'service_name');
    prevPort := ReadCfgValue(PrevDirValue, 'port');
    prevCfgDataDir := ReadDataDirFromConfig(PrevDirValue);
    if prevName <> '' then AppNameValue := prevName;
    if prevSvc <> '' then SvcValue := prevSvc;
    if prevPort <> '' then PortValue := prevPort;
    if prevCfgDataDir <> '' then DataDirValue := prevCfgDataDir;
    if DataDirValue = '' then
      DataDirValue := ExpandConstant('{commonappdata}\QinJiaGateway\data');

    if IsSilentInstall() then
      // 静默安装检测到已安装版本：自动升级（先停服务再覆盖文件、保留配置与数据）。
      // 否则 Inno 仍会沿用旧安装目录（UsePreviousAppDir），对运行中服务的 exe
      // 直接覆盖会因文件锁定导致安装永久卡死
      UpgradeMode := True
    else if MsgBox('检测到本软件已安装：' + PrevDirValue + #13#10#13#10
      + '是否更新到新版本？' + #13#10#13#10
      + '点击「是」：直接覆盖更新程序文件（不再逐步引导），现有配置与全部业务数据保持不变；'
      + #13#10 + '点击「否」：退出安装。',
      mbConfirmation, MB_YESNO) = IDNO then begin
      Result := False; Exit;
    end else
      UpgradeMode := True;
  end;

  // 非更新分支：数据目录沿用已装版本配置或默认值
  if DataDirValue = '' then begin
    if PrevDirValue <> '' then begin
      prevCfgDataDir := ReadDataDirFromConfig(PrevDirValue);
      if prevCfgDataDir <> '' then
        DataDirValue := prevCfgDataDir
      else
        DataDirValue := ExpandConstant('{commonappdata}\QinJiaGateway\data');
    end else
      DataDirValue := ExpandConstant('{commonappdata}\QinJiaGateway\data');
  end;

  // 命令行 /UPGRADE=1 时进入升级模式（与交互确认的 UpgradeMode 取或，不覆盖）
  UpgradeMode := UpgradeMode or (GetCmdParam('UPGRADE', '') = '1');
  Result := True;
end;

// 服务名合法性：仅字母/数字/下划线/短横线，长度1-64，不能以数字开头
function IsValidSvcName(s: String): Boolean;
var i: Integer; c: Char;
begin
  Result := False;
  s := Trim(s);
  if (Length(s) < 1) or (Length(s) > 64) then Exit;
  c := s[1];
  if ((c >= '0') and (c <= '9')) then Exit;
  for i := 1 to Length(s) do begin
    c := s[i];
    if not (((c >= 'A') and (c <= 'Z')) or ((c >= 'a') and (c <= 'z'))
      or ((c >= '0') and (c <= '9')) or (c = '_') or (c = '-')) then Exit;
  end;
  Result := True;
end;

// 规范化路径：去空白、正斜杠统一为反斜杠、转大写、去结尾反斜杠（保留 X:\ 根目录）
function NormalizePath(s: String): String;
begin
  Result := Trim(s);
  StringChange(Result, '/', '\');
  Result := AsciiUpper(Result);
  while (Length(Result) > 3) and (Result[Length(Result)] = '\') do
    Result := Copy(Result, 1, Length(Result) - 1);
end;

// 是否为本地盘符绝对路径（如 D:\xxx）
function IsAbsoluteLocalPath(s: String): Boolean;
var n: String;
begin
  n := NormalizePath(s);
  Result := (Length(n) >= 3)
    and (n[1] >= 'A') and (n[1] <= 'Z')
    and (n[2] = ':') and (n[3] = '\');
end;

// 判断 child 是否等于 parent 或真正位于 parent 目录内（按路径分隔符边界比较，
// 避免 D:\QinJiaGateway_data 被误判为 D:\QinJiaGateway 的子目录）
function IsPathInsideOrEqual(child, parent: String): Boolean;
var c, p: String;
begin
  c := NormalizePath(child);
  p := NormalizePath(parent);
  if c = p then begin
    Result := True; Exit;
  end;
  Result := (Length(c) >= Length(p) + 2)
    and (Copy(c, 1, Length(p) + 1) = p + '\');
end;

procedure InitializeWizard();
begin
  // 第零页（仅覆盖安装时显示）：提示已检测到的安装目录
  PrevDirPage := CreateCustomPage(wpWelcome, '安装目录', '检测到已安装的版本');

  PrevDirLbl := TNewStaticText.Create(PrevDirPage);
  PrevDirLbl.Parent := PrevDirPage.Surface;
  PrevDirLbl.Top := ScaleY(4);
  PrevDirLbl.Caption := '检测到本软件已安装，当前安装目录为：';

  PrevDirEdit := TNewEdit.Create(PrevDirPage);
  PrevDirEdit.Parent := PrevDirPage.Surface;
  PrevDirEdit.Top := ScaleY(26); PrevDirEdit.Width := ScaleX(360);
  PrevDirEdit.Text := PrevDirValue;
  PrevDirEdit.ReadOnly := True;
  PrevDirEdit.TabStop := False;
  PrevDirEdit.Color := clBtnFace;

  PrevDirTip := TNewStaticText.Create(PrevDirPage);
  PrevDirTip.Parent := PrevDirPage.Surface;
  PrevDirTip.Top := ScaleY(54); PrevDirTip.Width := ScaleX(400);
  PrevDirTip.Height := ScaleY(80);
  PrevDirTip.Caption := '继续安装将在该目录内覆盖更新程序文件，业务数据目录保持不变。'
    + #13#10 + '如需更改安装目录，请先从该目录运行卸载程序（或通过“控制面板 - 程序和功能”卸载）后再重新安装。'
    + #13#10 + #13#10 + '首次启动时将自动检测数据库：若表结构有变更将执行迁移，'
    + '服务器密码/私钥等敏感字段将自动加密存储，无需手动操作。';

  // 演示版无授权版本选择页（demo 模式跳过授权校验）

  // 第一页：软件名称设置
  NamePage := CreateCustomPage(wpSelectDir, '软件名称设置', '自定义平台显示名称与系统服务名');

  AppNameLbl := TNewStaticText.Create(NamePage);
  AppNameLbl.Parent := NamePage.Surface;
  AppNameLbl.Top := ScaleY(2); AppNameLbl.Caption := '软件名称（用于平台标题、快捷方式名称）：';
  AppNameEdit := TNewEdit.Create(NamePage);
  AppNameEdit.Parent := NamePage.Surface;
  AppNameEdit.Top := ScaleY(20); AppNameEdit.Width := ScaleX(330);
  AppNameEdit.Text := AppNameValue;

  SvcLbl := TNewStaticText.Create(NamePage);
  SvcLbl.Parent := NamePage.Surface;
  SvcLbl.Top := ScaleY(52); SvcLbl.Caption := '系统服务名（仅英文字母、数字、_ 或 -，不能以数字开头）：';
  SvcEdit := TNewEdit.Create(NamePage);
  SvcEdit.Parent := NamePage.Surface;
  SvcEdit.Top := ScaleY(70); SvcEdit.Width := ScaleX(240);
  SvcEdit.Text := SvcValue;

  // 第二页：数据目录设置
  DataPage := CreateInputDirPage(NamePage.ID,
    '数据目录设置', '选择业务数据的保存位置',
    '平台的服务器列表、账号、配置等数据将保存到该目录（卸载时保留）。',
    False, '');
  DataPage.Add('数据目录：');
  DataPage.Values[0] := DataDirValue;

  // 第三页：端口设置（演示版管理员账号固定为 demo/demo123，不可修改）
  SettingsPage := CreateCustomPage(DataPage.ID, '端口设置', '配置 Web 服务端口');

  PortLbl := TNewStaticText.Create(SettingsPage);
  PortLbl.Parent := SettingsPage.Surface;
  PortLbl.Top := ScaleY(2); PortLbl.Caption := '服务端口（1-65535）：';
  PortEdit := TNewEdit.Create(SettingsPage);
  PortEdit.Parent := SettingsPage.Surface;
  PortEdit.Top := ScaleY(20); PortEdit.Width := ScaleX(130); PortEdit.Text := PortValue;

  DemoAcctLbl := TNewStaticText.Create(SettingsPage);
  DemoAcctLbl.Parent := SettingsPage.Surface;
  DemoAcctLbl.Top := ScaleY(56); DemoAcctLbl.Width := ScaleX(400);
  DemoAcctLbl.Height := ScaleY(60);
  DemoAcctLbl.Caption := '演示账号（固定，安装后不可修改）：' + #13#10
    + '用户名：demo' + #13#10 + '密码：demo123';

  TipLbl := TNewStaticText.Create(SettingsPage);
  TipLbl.Parent := SettingsPage.Surface;
  TipLbl.Top := ScaleY(120); TipLbl.Width := ScaleX(400);
  TipLbl.Caption := '提示：演示版内置 6 台模拟服务器与监控网址，无需真实服务器即可体验全部界面功能。';
  TipLbl.Font.Style := TipLbl.Font.Style + [fsItalic];

  // [Icons] 在 PostInstall 之前生成，需提前锁定值
  AppNameValue := Trim(AppNameEdit.Text);
  SvcValue := Trim(SvcEdit.Text);
  PortValue := Trim(PortEdit.Text);
end;

// 覆盖安装提示页仅在检测到旧安装目录时显示
function ShouldSkipPage(PageID: Integer): Boolean;
begin
  // 升级模式：跳过所有向导页（含欢迎/许可/目录/设置/就绪/完成），
  // 直接进入 wpInstalling 显示安装进度，安装完成后由 ssPostInstall 弹提示
  if UpgradeMode then begin
    Result := True;
    Exit;
  end;
  if PageID = PrevDirPage.ID then
    Result := (Trim(PrevDirValue) = '')
  else
    Result := False;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var p: Integer;
begin
  Result := True;
  if CurPageID = NamePage.ID then begin
    if Trim(AppNameEdit.Text) = '' then begin
      MsgBox('软件名称不能为空。', mbError, MB_OK);
      Result := False; Exit;
    end;
    if not IsValidSvcName(SvcEdit.Text) then begin
      MsgBox('系统服务名只能包含英文字母、数字、下划线(_)或短横线(-)，长度1-64位，且不能以数字开头。', mbError, MB_OK);
      Result := False; Exit;
    end;
    AppNameValue := Trim(AppNameEdit.Text);
    SvcValue := Trim(SvcEdit.Text);
  end;
  if CurPageID = DataPage.ID then begin
    DataDirValue := Trim(DataPage.Values[0]);
    if DataDirValue = '' then begin
      MsgBox('数据目录不能为空。', mbError, MB_OK);
      Result := False; Exit;
    end;
    if not IsAbsoluteLocalPath(DataDirValue) then begin
      MsgBox('数据目录必须是完整的绝对路径（例如 D:\QinJiaGatewayData）。', mbError, MB_OK);
      Result := False; Exit;
    end;
    if IsPathInsideOrEqual(DataDirValue, ExpandConstant('{app}')) then begin
      MsgBox('数据目录不能位于程序安装目录内，避免卸载程序时丢失数据。', mbError, MB_OK);
      Result := False; Exit;
    end;
  end;
  if CurPageID = SettingsPage.ID then begin
    p := StrToIntDef(Trim(PortEdit.Text), -1);
    if (p < 1) or (p > 65535) then begin
      MsgBox('请输入 1-65535 之间的有效端口号。', mbError, MB_OK);
      Result := False; Exit;
    end;
    PortValue := Trim(PortEdit.Text);
  end;
end;

// [Setup]/[Icons] 常量函数
function GetAppName(Param: String): String;
begin
  if AppNameValue <> '' then Result := AppNameValue else Result := '{#DefaultAppName}';
end;

// [Icons] 段读取端口
function GetPort(Param: String): String;
begin
  if PortValue <> '' then Result := PortValue else Result := '8080';
end;

// 生成随机 JWT 密钥（48位十六进制）
function MakeSecret: String;
var i: Integer; hex: String;
begin
  hex := '0123456789abcdef';
  Result := 'qj-';
  for i := 0 to 47 do
    Result := Result + hex[Random(16) + 1];
end;

// YAML 双引号字符串转义（名称中如含 " 或 \）
function YamlQuote(s: String): String;
begin
  StringChange(s, '\', '\\');
  StringChange(s, '"', '\"');
  Result := '"' + s + '"';
end;

// PrepareToInstall 覆盖文件前的准备：升级模式下先停止服务，
// 否则 gateway.exe 被占用导致文件替换失败。sc stop 会等待服务真正停止后才返回。
function PrepareToInstall(var NeedsRestart: Boolean): String;
var rc: Integer;
begin
  Result := '';
  if UpgradeMode then begin
    Log('upgrade mode: stopping service ' + SvcValue);
    Exec(ExpandConstant('{sys}\sc.exe'), 'stop ' + SvcValue,
      '', SW_HIDE, ewWaitUntilTerminated, rc);
  end;
end;

// 安装完成提示窗体（前向声明，实现在 CurStepChanged 之后）
function ShowCompletionForm(): Boolean; forward;

procedure CurStepChanged(CurStep: TSetupStep);
var binPath, dataDir, dsnDir, url: String; rc: Integer; Lines: TArrayOfString;
    keepConfig: Boolean;
begin
  if CurStep <> ssPostInstall then Exit;

  // 从向导控件锁定最终值（静默安装时控件文本来自命令行参数）
  AppNameValue := Trim(AppNameEdit.Text);
  SvcValue := Trim(SvcEdit.Text);
  PortValue := Trim(PortEdit.Text);
  DataDirValue := Trim(DataPage.Values[0]);
  if DataDirValue = '' then
    DataDirValue := ExpandConstant('{commonappdata}\QinJiaGateway\data');

  dataDir := ExpandConstant(DataDirValue);
  // 去掉结尾反斜杠（保留盘符根目录如 D:\）
  while (Length(dataDir) > 3) and (dataDir[Length(dataDir)] = '\') do
    dataDir := Copy(dataDir, 1, Length(dataDir) - 1);
  ForceDirectories(dataDir);

  // 升级模式且原 config.yaml 存在 → 保留配置原样（不重置 JWT 密钥）
  keepConfig := UpgradeMode and FileExists(ExpandConstant('{app}\config.yaml'));

  // 生成 config.yaml（UTF-8 无 BOM，避免中文导致 YAML 解析失败）
  // 演示版：demo: true 启用演示模式（内置模拟服务器/命令/监控网址）；
  // 管理员 demo 账号由程序启动时自举创建（demo/demo123，不可修改），无需安装程序写入
  if not keepConfig then begin
    SetArrayLength(Lines, 18);
  Lines[0] := '# ' + AppNameValue + ' - 由安装程序生成';
  Lines[1] := 'demo: true';
  Lines[2] := 'app:';
  Lines[3] := '  name: ' + YamlQuote(AppNameValue);
  Lines[4] := '  service_name: ' + SvcValue;
  Lines[5] := 'data_dir: ' + YamlQuote(DataDirValue);
  Lines[6] := 'server:';
  Lines[7] := '  port: ' + PortValue;
  Lines[8] := '  mode: release';
  Lines[9] := 'database:';
  Lines[10] := '  driver: sqlite';
  // dsn 用正斜杠绝对路径（Go 在 Windows 上兼容），避免 YAML 双引号内反斜杠转义问题
  dsnDir := dataDir;
  StringChange(dsnDir, '\', '/');
  Lines[11] := '  dsn: "' + dsnDir + '/gateway.db"';
  Lines[12] := 'jwt:';
  Lines[13] := '  secret: "' + MakeSecret() + '"';
  Lines[14] := '  expire: 7200';
  Lines[15] := '  refresh_expire: 604800';
  Lines[16] := 'license:';
  Lines[17] := '  edition: demo';
  SaveStringsToUTF8FileWithoutBOM(ExpandConstant('{app}\config.yaml'), Lines, False);
  end;

  try
    if UpgradeMode then begin
      // 升级模式：服务已注册（PrepareToInstall 中已停止），直接启动即可
      if Exec(ExpandConstant('{sys}\sc.exe'), 'start ' + SvcValue,
        '', SW_HIDE, ewWaitUntilTerminated, rc) then
          Log('upgrade: sc start rc=' + IntToStr(rc));
    end else begin
      // 全新/普通安装：注册 Windows 系统服务（开机自启 + 失败自动重启）
      // binPath 内嵌引号必须用 \" 转义，否则 sc 解析参数失败(1639)
      binPath := '\"' + ExpandConstant('{app}\gateway.exe') + '\" -config \"'
        + ExpandConstant('{app}\config.yaml') + '\" -data \"' + dataDir + '\"';
      Log('binPath = ' + binPath);
      if Exec(ExpandConstant('{sys}\sc.exe'),
        'create ' + SvcValue + ' binPath= "' + binPath + '" start= auto displayname= ' + YamlQuote(AppNameValue),
        '', SW_HIDE, ewWaitUntilTerminated, rc) then
          Log('sc create rc=' + IntToStr(rc))
      else
        Log('sc create Exec returned FALSE');
      Exec(ExpandConstant('{sys}\sc.exe'),
        'description ' + SvcValue + ' ' + YamlQuote(AppNameValue + '后台服务'),
        '', SW_HIDE, ewWaitUntilTerminated, rc);
      Exec(ExpandConstant('{sys}\sc.exe'),
        'failure ' + SvcValue + ' reset= 60 actions= restart/5000/restart/5000/restart/5000',
        '', SW_HIDE, ewWaitUntilTerminated, rc);
      if Exec(ExpandConstant('{sys}\sc.exe'), 'start ' + SvcValue,
        '', SW_HIDE, ewWaitUntilTerminated, rc) then
          Log('sc start rc=' + IntToStr(rc));
    end;

    // 防火墙放行：全新安装与升级模式都必须执行——
    // 旧版本安装包可能未加规则或只加了端口规则；升级时端口也可能变更。
    // 先删后加保证幂等，不会重复堆积规则。
    // 端口规则（放行 TCP 端口）+ 程序规则（gateway.exe 整体放行）：
    // 「允许的应用」控制面板列表只显示程序规则，端口规则对用户不可见，两者缺一不可。
    Exec(ExpandConstant('{sys}\netsh.exe'),
      'advfirewall firewall delete rule name="' + SvcValue + '"',
      '', SW_HIDE, ewWaitUntilTerminated, rc);
    Exec(ExpandConstant('{sys}\netsh.exe'),
      'advfirewall firewall delete rule name="' + SvcValue + '_app"',
      '', SW_HIDE, ewWaitUntilTerminated, rc);
    Exec(ExpandConstant('{sys}\netsh.exe'),
      'advfirewall firewall add rule name="' + SvcValue + '" dir=in action=allow protocol=TCP localport=' + PortValue + ' profile=any',
      '', SW_HIDE, ewWaitUntilTerminated, rc);
    Exec(ExpandConstant('{sys}\netsh.exe'),
      'advfirewall firewall add rule name="' + SvcValue + '_app" dir=in action=allow program="' + ExpandConstant('{app}\gateway.exe') + '" enable=yes profile=any',
      '', SW_HIDE, ewWaitUntilTerminated, rc);
  except
    Log('EXCEPTION during service setup: ' + GetExceptionMessage);
  end;

  // 安装/更新完成后提示用户；仅全自动极静部署 /VERYSILENT 时跳过。
  // 交互覆盖升级（非静默）与 /SILENT 均弹出，让用户知道安装已完成。
  // 使用复选框而非「是/否/取消」：可同时勾选打开 Web 平台与管理工具。
  if not IsVerySilentInstall() then begin
    url := 'http://localhost:' + PortValue + '/';
    if ShowCompletionForm() then begin
      if CompletionOpenWeb then
        ShellExec('', url, '', '', SW_SHOWNORMAL, ewNoWait, rc);
      if CompletionOpenMgr then
        ShellExec('', ExpandConstant('{sys}\wscript.exe'),
          '"' + ExpandConstant('{app}\manager.vbs') + '"', '', SW_SHOWNORMAL, ewNoWait, rc);
    end;
  end;
end;

// 安装完成提示窗体：两个可多选的复选框 + 确定按钮。
// 返回 True 表示用户点击了确定（按勾选执行操作），False 表示关闭窗体不执行。
// 注意：本机 Inno Setup 为 6.6.0+，CreateCustomForm 签名已变为
// (ClientWidth, ClientHeight, KeepSizeX, KeepSizeY)，且不可直接 TSetupForm.Create
// （会报 "Resource TSetupForm not found"）。控件须用 TNew 系列主题控件。
function ShowCompletionForm(): Boolean;
var
  Form: TSetupForm;
  Lbl: TNewStaticText;
  ChkWeb, ChkMgr: TNewCheckBox;
  BtnOK: TNewButton;
  mr: Integer;
begin
  Log('completion form: begin');
  Result := False;
  try
    Form := CreateCustomForm(ScaleX(340), ScaleY(140), False, False);
    try
      Form.Caption := '安装完成';

      Lbl := TNewStaticText.Create(Form);
      Lbl.Parent := Form;
      Lbl.Top := ScaleY(16);
      Lbl.Left := ScaleX(16);
      Lbl.Caption := '安装完成，请选择要执行的操作：';

      ChkWeb := TNewCheckBox.Create(Form);
      ChkWeb.Parent := Form;
      ChkWeb.Top := ScaleY(48);
      ChkWeb.Left := ScaleX(24);
      ChkWeb.Width := ScaleX(300);
      ChkWeb.Caption := '在浏览器中打开 Web 平台';
      ChkWeb.Checked := True;

      ChkMgr := TNewCheckBox.Create(Form);
      ChkMgr.Parent := Form;
      ChkMgr.Top := ScaleY(76);
      ChkMgr.Left := ScaleX(24);
      ChkMgr.Width := ScaleX(300);
      ChkMgr.Caption := '打开服务管理工具';
      ChkMgr.Checked := True;

      BtnOK := TNewButton.Create(Form);
      BtnOK.Parent := Form;
      BtnOK.Top := Form.ClientHeight - ScaleY(33);
      BtnOK.Left := Form.ClientWidth - ScaleX(91);
      BtnOK.Width := ScaleX(75);
      BtnOK.Height := ScaleY(23);
      BtnOK.Caption := '确定';
      BtnOK.ModalResult := mrOk;
      BtnOK.Default := True;

      Form.ActiveControl := ChkWeb;
      Form.FlipAndCenterIfNeeded(True, WizardForm, False);
      Log('completion form: before ShowModal');

      mr := Form.ShowModal();
      Log('completion form: ShowModal returned ' + IntToStr(mr));
      Result := (mr = mrOk);
      if Result then begin
        CompletionOpenWeb := ChkWeb.Checked;
        CompletionOpenMgr := ChkMgr.Checked;
      end;
    finally
      Form.Free();
    end;
  except
    Log('completion form EXCEPTION: ' + GetExceptionMessage);
  end;
end;

// 卸载前从 config.yaml 提取 service_name（纯ASCII，无编码问题）
function ReadSvcFromConfig(): String;
var Lines: TArrayOfString; i, p: Integer; line: String;
begin
  Result := '{#DefaultSvcName}';
  if not FileExists(ExpandConstant('{app}\config.yaml')) then Exit;
  if LoadStringsFromFile(ExpandConstant('{app}\config.yaml'), Lines) then begin
    for i := 0 to GetArrayLength(Lines) - 1 do begin
      line := Lines[i];
      if Pos('service_name:', line) > 0 then begin
        p := Pos(':', line);
        Result := Trim(Copy(line, p + 1, MaxInt));
        Exit;
      end;
    end;
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var rc: Integer; svc: String;
begin
  if CurUninstallStep <> usUninstall then Exit;
  // 停止并删除系统服务、移除防火墙规则（业务数据目录保留）
  svc := ReadSvcFromConfig();
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop ' + svc,
    '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\sc.exe'), 'delete ' + svc,
    '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\netsh.exe'),
    'advfirewall firewall delete rule name="' + svc + '"',
    '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\netsh.exe'),
    'advfirewall firewall delete rule name="' + svc + '_app"',
    '', SW_HIDE, ewWaitUntilTerminated, rc);
end;
