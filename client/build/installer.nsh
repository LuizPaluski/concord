!macro customInstall
  ; Sobras da versão Wails, que instalava na mesma pasta. Sem isso aparecem dois
  ; Concord em Aplicativos, e o desinstalador velho apagaria o app novo.
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ConcordConcord"
  Delete "$INSTDIR\uninstall.exe"

  ; O atualizador da versão Wails roda este instalador com /S e não reabre o app.
  ; As atualizações do Electron também abrem; a trava de instância única evita a duplicata.
  IfSilent 0 +2
    Exec '"$INSTDIR\${APP_EXECUTABLE_FILENAME}"'
!macroend
