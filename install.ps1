# Ставит gitone на Windows одной командой:
#
#   irm https://raw.githubusercontent.com/oberemokis/gitone/main/install.ps1 | iex
#
# Переменные окружения:
#   GITONE_VERSION      версия, например v0.4.0; по умолчанию последняя
#   GITONE_INSTALL_DIR  каталог; по умолчанию %LOCALAPPDATA%\Programs\gitone
#   GITONE_RELEASES_URL адрес релизов, его подменяет CI локальным сервером
#
# Скрипт ставит версии с v0.4.0: в более старых релизах нет сборок под
# Windows. PATH он не меняет.
#
# Тело в блоке, ошибки через throw: через iex скрипт идёт в сессии
# пользователя, exit закрыл бы его окно, а настройки остались бы в ней.

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    # Windows PowerShell 5.1 сам не включает TLS 1.2, а без него GitHub не отвечает
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $base = if ($env:GITONE_RELEASES_URL) { $env:GITONE_RELEASES_URL } else { 'https://github.com/oberemokis/gitone/releases' }
    $dir = if ($env:GITONE_INSTALL_DIR) { $env:GITONE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\gitone' }

    # 32-битный PowerShell на 64-битной Windows видит x86, настоящая архитектура
    # лежит в PROCESSOR_ARCHITEW6432
    $machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    $arch = switch ($machine) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "gitone install: unsupported architecture: $machine" }
    }
    $asset = "gitone_windows_$arch.exe"

    if ($env:GITONE_VERSION) {
        $version = $env:GITONE_VERSION
        if (-not $version.StartsWith('v')) { $version = "v$version" }
        $url = "$base/download/$version"
    } else {
        $url = "$base/latest/download"
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('gitone-' + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$url/$asset" -OutFile (Join-Path $tmp $asset)
            Invoke-WebRequest -UseBasicParsing -Uri "$url/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
        } catch {
            throw "gitone install: cannot download $url/$asset (only gitone v0.4.0 and later install this way): $($_.Exception.Message)"
        }

        # checksums.txt перечисляет все файлы релиза, сверяется строка своего
        $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { ($_ -split '\s+')[1] -eq $asset } | Select-Object -First 1
        if (-not $line) { throw "gitone install: checksums.txt has no line for $asset, nothing was installed" }
        $want = ($line -split '\s+')[0]
        $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash.ToLower()
        if ($got -ne $want) { throw "gitone install: checksum of $asset does not match checksums.txt, nothing was installed" }

        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $target = Join-Path $dir 'gitone.exe'
        $staged = Join-Path $dir 'gitone.exe.new'
        Copy-Item (Join-Path $tmp $asset) $staged -Force
        try {
            Move-Item $staged $target -Force
        } catch {
            Remove-Item $staged -Force -ErrorAction SilentlyContinue
            throw "gitone install: cannot replace $target, close running gitone and try again: $($_.Exception.Message)"
        }

        $installed = & $target --version
        Write-Host "$installed installed to $target"

        $paths = ([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') + ($env:Path -split ';')
        if ($paths -notcontains $dir) {
            Write-Host "$dir is not in PATH, add it in Settings > System > About > Advanced system settings > Environment Variables, then open a new terminal"
        }
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}
