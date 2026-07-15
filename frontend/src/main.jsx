import React, { useCallback, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Copy,
  ExternalLink,
  FolderOpen,
  Languages,
  LogIn,
  LogOut,
  Play,
  Settings,
  Square,
} from "lucide-react";
import {
  Login,
  Logout,
  OpenStandaloneWebURL,
  OpenWebURL,
  SelectRootDirectory,
  SelectStandaloneRootDirectory,
  SetAPIBaseURL,
  StartFileProxy,
  StartFileProxyWeb,
  StartFileProxyWebStandalone,
  Status,
  StopFileProxy,
  StopFileProxyWeb,
  StopFileProxyWebStandalone,
} from "../wailsjs/go/main/App";
import { ClipboardSetText } from "../wailsjs/runtime/runtime";
import logo from "./assets/logo.png";
import "./styles.css";

const translations = {
  en: {
    settings: "Settings",
    apiDomain: "API Domain",
    save: "Save",
    cancel: "Cancel",
    login: "Log in to continue",
    username: "Username",
    password: "Password",
    authenticatorCode: "Authenticator code",
    authenticatorCodeHint:
      "Enter the 6-digit code from your authenticator app.",
    logout: "Log out",
    account: "Account",
    worker: "File Service",
    client: "Client",
    standalone: "Standalone Web",
    root: "Root directory",
    selectDirectory: "Select directory",
    threads: "Worker threads",
    allowDelete: "Allow file deletion",
    port: "Port",
    autoOpenBrowser: "Open browser when ready",
    browserAddress: "Local address",
    startsOnLocalhost: "Available on localhost after start",
    start: "Start service",
    stop: "Stop service",
    running: "Running",
    stopped: "Stopped",
    logs: "Logs",
    noLogs: "No logs yet.",
    working: "Working",
    openWebAddress: "Open local address",
    copyWebAddress: "Copy local address",
    singleWorker: "Only one file-service instance can run at a time.",
    workerAlreadyRunning: "The file service is already running.",
    loginTask: "login",
    logoutTask: "logout",
    selectTask: "select directory",
    selectStandaloneTask: "select standalone directory",
    startWorkerTask: "start file service",
    stopWorkerTask: "stop file service",
    startClientTask: "start client",
    stopClientTask: "stop client",
    startStandaloneTask: "start standalone web",
    stopStandaloneTask: "stop standalone web",
    openWebTask: "open web address",
    openStandaloneTask: "open standalone web address",
    saveSettingsTask: "save settings",
    serviceOverview: "Overview",
    howItWorks: "How it works",
    securityNotes: "Access and safety",
    setup: "Service setup",
    loginRequired: "Sign in to configure and start this service.",
    workerSummary: "Share one selected folder through your workspace.",
    workerFlow:
      "Sign in, select a local root directory, then start the managed file worker.",
    workerSafety:
      "The selected root defines the accessible files. Deletion remains disabled unless you explicitly allow it.",
    workerSteps: [
      "Sign in to your account.",
      "Choose the folder to expose.",
      "Set threads and deletion permission, then start.",
    ],
    clientSummary:
      "Open the local browser gateway for your file service.",
    clientFlow:
      "The client connects your signed-in workspace to a browser interface running on this computer.",
    clientSafety:
      "The browser gateway listens only on localhost. Start the file service first so its files are available.",
    clientSteps: [
      "Sign in to your account.",
      "Start the file service if it is not running.",
      "Choose a local port and start the browser gateway.",
    ],
    standaloneSummary:
      "Browse and manage a local folder directly in your browser.",
    standaloneFlow:
      "This mode serves the selected folder without an account, certificates, or a running file service.",
    standaloneSafety:
      "The browser interface listens only on localhost. Enable deletion only for folders you are prepared to modify.",
    standaloneSteps: [
      "Choose the folder to manage.",
      "Choose a local port and deletion permission.",
      "Start the web interface and open its local address.",
    ],
  },
  zh: {
    settings: "设置",
    apiDomain: "API 域名",
    save: "保存",
    cancel: "取消",
    login: "登录后继续",
    username: "用户名",
    password: "密码",
    authenticatorCode: "身份验证器验证码",
    authenticatorCodeHint: "请输入 Google 身份验证器中的 6 位验证码。",
    logout: "退出登录",
    account: "账户",
    worker: "文件服务",
    client: "客户端",
    standalone: "单机 Web",
    root: "根目录",
    selectDirectory: "选择目录",
    threads: "工作线程数",
    allowDelete: "允许删除文件",
    port: "端口",
    autoOpenBrowser: "启动后自动打开浏览器",
    browserAddress: "本机访问地址",
    startsOnLocalhost: "启动后可通过 localhost 访问",
    start: "启动服务",
    stop: "停止服务",
    running: "运行中",
    stopped: "已停止",
    logs: "日志",
    noLogs: "暂无日志。",
    working: "处理中",
    openWebAddress: "打开本机地址",
    copyWebAddress: "复制本机地址",
    singleWorker: "文件服务一次仅支持运行一个实例。",
    workerAlreadyRunning: "文件服务已在运行。",
    loginTask: "登录",
    logoutTask: "退出登录",
    selectTask: "选择目录",
    selectStandaloneTask: "选择单机目录",
    startWorkerTask: "启动文件服务",
    stopWorkerTask: "停止文件服务",
    startClientTask: "启动客户端",
    stopClientTask: "停止客户端",
    startStandaloneTask: "启动单机 Web",
    stopStandaloneTask: "停止单机 Web",
    openWebTask: "打开访问地址",
    openStandaloneTask: "打开单机 Web 地址",
    saveSettingsTask: "保存设置",
    serviceOverview: "服务说明",
    howItWorks: "工作方式",
    securityNotes: "访问与安全",
    setup: "服务配置",
    loginRequired: "登录后即可配置并启动此服务。",
    workerSummary: "将选定目录作为当前工作区中的受控文件服务。",
    workerFlow: "登录后选择本机根目录，再启动受管理的文件服务进程。",
    workerSafety:
      "根目录决定可访问的文件范围；除非明确开启，否则不允许删除文件。",
    workerSteps: [
      "登录账户。",
      "选择需要提供访问的目录。",
      "设置线程数和删除权限后启动。",
    ],
    clientSummary: "为文件服务启动本机浏览器访问入口。",
    clientFlow: "客户端会连接已登录的工作区，并在此电脑上启动浏览器管理界面。",
    clientSafety:
      "浏览器入口仅监听 localhost。请先启动文件服务，才能访问其中的文件。",
    clientSteps: [
      "登录账户。",
      "确认文件服务已经启动。",
      "选择本机端口后启动浏览器入口。",
    ],
    standaloneSummary: "直接在浏览器中管理一个本机目录。",
    standaloneFlow: "此模式不需要账户、证书或正在运行的文件服务。",
    standaloneSafety:
      "浏览器入口仅监听 localhost。只应为允许修改的目录开启删除权限。",
    standaloneSteps: [
      "选择需要管理的目录。",
      "设置本机端口和删除权限。",
      "启动 Web 界面并打开本机地址。",
    ],
  },
};

const emptyStatus = {
  api_base_url: "https://iot.huabot.com",
  logged_in: false,
  root_dir: "",
  start_options: null,
  running: false,
  web_running: false,
  web_url: "",
  standalone_root_dir: "",
  standalone_start_options: null,
  standalone_web_running: false,
  standalone_web_url: "",
  logs: [],
  last_error: "",
  binary_target: "",
};

function App() {
  const [status, setStatus] = useState(emptyStatus);
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [totpRequired, setTotpRequired] = useState(false);
  const [thread, setThread] = useState(4);
  const [allowDelete, setAllowDelete] = useState(false);
  const [port, setPort] = useState(8080);
  const [autoOpenBrowser, setAutoOpenBrowser] = useState(true);
  const [standaloneAllowDelete, setStandaloneAllowDelete] = useState(false);
  const [standalonePort, setStandalonePort] = useState(8081);
  const [standaloneAutoOpenBrowser, setStandaloneAutoOpenBrowser] =
    useState(true);
  const [serviceTab, setServiceTab] = useState("standalone");
  const [apiBaseURL, setAPIBaseURL] = useState(emptyStatus.api_base_url);
  const [language, setLanguage] = useState(
    () =>
      window.localStorage.getItem("myna-file-proxy-language") ||
      (navigator.language.startsWith("zh") ? "zh" : "en"),
  );
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const t = useCallback(
    (key) => translations[language][key] || translations.en[key] || key,
    [language],
  );
  const refresh = useCallback(async () => setStatus(await Status()), []);

  useEffect(() => {
    void refresh().catch((err) => setError(String(err)));
  }, [refresh]);
  useEffect(() => {
    if (settingsOpen) return;
    const options = status.start_options || {};
    setThread(options.thread || 4);
    setAllowDelete(Boolean(options.allow_delete));
    setPort(options.port || 8080);
    setAutoOpenBrowser(options.auto_open_browser !== false);
    const standaloneOptions = status.standalone_start_options || {};
    setStandaloneAllowDelete(Boolean(standaloneOptions.allow_delete));
    setStandalonePort(standaloneOptions.port || 8081);
    setStandaloneAutoOpenBrowser(standaloneOptions.auto_open_browser !== false);
    setAPIBaseURL(status.api_base_url || emptyStatus.api_base_url);
  }, [settingsOpen, status]);
  useEffect(
    () => window.localStorage.setItem("myna-file-proxy-language", language),
    [language],
  );

  const run = useCallback(
    async (label, fn) => {
      setBusy(label);
      setError("");
      try {
        const next = await fn();
        setStatus(next);
        return next;
      } catch (err) {
        setError(err?.message || String(err));
        await refresh();
        return null;
      } finally {
        setBusy("");
      }
    },
    [refresh],
  );
  const login = useCallback(async () => {
    try {
      return await Login(name, password, totpCode);
    } catch (err) {
      const message = err?.message || String(err);
      if (message === "totp required" || message === "totp invalid")
        setTotpRequired(true);
      throw err;
    }
  }, [name, password, totpCode]);
  const options = useMemo(
    () => ({
      thread,
      allow_delete: allowDelete,
      port,
      auto_open_browser: autoOpenBrowser,
    }),
    [allowDelete, autoOpenBrowser, port, thread],
  );
  const standaloneOptions = useMemo(
    () => ({
      allow_delete: standaloneAllowDelete,
      port: standalonePort,
      auto_open_browser: standaloneAutoOpenBrowser,
    }),
    [standaloneAllowDelete, standaloneAutoOpenBrowser, standalonePort],
  );
  const displayName =
    status.user_info?.nick_name || status.user_info?.name || status.user_name;
  const avatarUrl = status.user_info?.avatar_url || "";
  const avatarFallback = (displayName || status.user_name || "U")
    .slice(0, 1)
    .toUpperCase();

  return (
    <main className="shell">
      <header className="topbar">
        <div className="brand">
          <img src={logo} alt="" />
          <div>
            <h1>
              Myna File Proxy <span className="versionBadge">v1.2.1.0</span>
            </h1>
            <p>{status.api_base_url}</p>
          </div>
        </div>
        <div className="topActions">
          <div className="languageSwitch">
            <Languages size={16} />
            <button
              className={language === "en" ? "active" : ""}
              type="button"
              onClick={() => setLanguage("en")}
            >
              EN
            </button>
            <button
              className={language === "zh" ? "active" : ""}
              type="button"
              onClick={() => setLanguage("zh")}
            >
              中文
            </button>
          </div>
          <button
            className="iconButton"
            type="button"
            onClick={() => setSettingsOpen((open) => !open)}
            disabled={!!busy}
            title={t("settings")}
          >
            <Settings size={18} />
          </button>
        </div>
      </header>
      {settingsOpen && (
        <SettingsPanel
          status={status}
          t={t}
          busy={busy}
          apiBaseURL={apiBaseURL}
          setAPIBaseURL={setAPIBaseURL}
          avatarUrl={avatarUrl}
          avatarFallback={avatarFallback}
          displayName={displayName}
          onCancel={() => {
            setAPIBaseURL(status.api_base_url || emptyStatus.api_base_url);
            setSettingsOpen(false);
          }}
          onSave={() =>
            run(t("saveSettingsTask"), async () => {
              const next = await SetAPIBaseURL(apiBaseURL);
              setSettingsOpen(false);
              return next;
            })
          }
          onLogout={() =>
            run(t("logoutTask"), async () => {
              const next = await Logout();
              setSettingsOpen(false);
              return next;
            })
          }
        />
      )}
      <nav className="serviceSwitch" aria-label="Services">
        <button
          className={serviceTab === "worker" ? "active" : ""}
          type="button"
          onClick={() => setServiceTab("worker")}
        >
          {t("worker")}
        </button>
        <button
          className={serviceTab === "client" ? "active" : ""}
          type="button"
          onClick={() => setServiceTab("client")}
        >
          {t("client")}
        </button>
        <button
          className={serviceTab === "standalone" ? "active" : ""}
          type="button"
          onClick={() => setServiceTab("standalone")}
        >
          {t("standalone")}
        </button>
      </nav>
      <section className="overviewGrid singleService">
        {serviceTab === "worker" && (
          <ServicePanel
            icon={<FolderOpen size={20} />}
            title={t("worker")}
            running={status.running}
            t={t}
            service="worker"
          >
            <ServiceIntro service="worker" t={t} />
            {status.logged_in ? (
              <>
                <div
                  className={
                    status.running ? "workerLimit active" : "workerLimit"
                  }
                >
                  {status.running
                    ? t("workerAlreadyRunning")
                    : t("singleWorker")}
                </div>
                <section className="setupSection">
                  <h3>{t("setup")}</h3>
                  <StatusRow
                    label={t("root")}
                    ready={!!status.root_dir}
                    value={status.root_dir || t("selectDirectory")}
                  />
                  <div className="fieldGrid">
                    <label>
                      {t("threads")}
                      <input
                        className="shortNumberInput"
                        type="number"
                        min="1"
                        max="16"
                        value={thread}
                        onChange={(event) =>
                          setThread(Number(event.target.value || 4))
                        }
                      />
                    </label>
                    <label className="checkLine">
                      <input
                        type="checkbox"
                        checked={allowDelete}
                        onChange={(event) =>
                          setAllowDelete(event.target.checked)
                        }
                      />
                      {t("allowDelete")}
                    </label>
                  </div>
                </section>
                <div className="buttonRow serviceActions">
                  <button
                    className="iconButton"
                    type="button"
                    disabled={!!busy || status.running}
                    onClick={() => run(t("selectTask"), SelectRootDirectory)}
                    title={t("selectDirectory")}
                  >
                    <FolderOpen size={18} />
                  </button>
                  <button
                    className="primary"
                    type="button"
                    disabled={!status.root_dir || status.running || !!busy}
                    onClick={() =>
                      run(t("startWorkerTask"), () => StartFileProxy(options))
                    }
                  >
                    <Play size={18} />
                    {t("start")}
                  </button>
                  <button
                    type="button"
                    disabled={!status.running || !!busy}
                    onClick={() => run(t("stopWorkerTask"), StopFileProxy)}
                  >
                    <Square size={18} />
                    {t("stop")}
                  </button>
                </div>
              </>
            ) : (
              <LoginCard
                t={t}
                busy={busy}
                name={name}
                password={password}
                totpCode={totpCode}
                totpRequired={totpRequired}
                setName={setName}
                setPassword={setPassword}
                setTotpCode={setTotpCode}
                onSubmit={() => run(t("loginTask"), login)}
              />
            )}
          </ServicePanel>
        )}
        {serviceTab === "client" && (
          <ServicePanel
            icon={<ExternalLink size={20} />}
            title={t("client")}
            running={status.web_running}
            t={t}
            service="client"
          >
            <ServiceIntro service="client" t={t} />
            {status.logged_in ? (
              <>
                <section className="setupSection">
                  <h3>{t("setup")}</h3>
                  <WebControls
                    running={status.web_running}
                    url={status.web_url}
                    port={port}
                    setPort={setPort}
                    autoOpenBrowser={autoOpenBrowser}
                    setAutoOpenBrowser={setAutoOpenBrowser}
                    busy={busy}
                    t={t}
                    onOpen={() => run(t("openWebTask"), OpenWebURL)}
                    onCopy={() =>
                      ClipboardSetText(status.web_url).catch((err) =>
                        setError(String(err)),
                      )
                    }
                    onStart={() =>
                      run(t("startClientTask"), () =>
                        StartFileProxyWeb(options),
                      )
                    }
                    onStop={() => run(t("stopClientTask"), StopFileProxyWeb)}
                  />
                </section>
              </>
            ) : (
              <LoginCard
                t={t}
                busy={busy}
                name={name}
                password={password}
                totpCode={totpCode}
                totpRequired={totpRequired}
                setName={setName}
                setPassword={setPassword}
                setTotpCode={setTotpCode}
                onSubmit={() => run(t("loginTask"), login)}
              />
            )}
          </ServicePanel>
        )}
        {serviceTab === "standalone" && (
          <ServicePanel
            icon={<ExternalLink size={20} />}
            title={t("standalone")}
            running={status.standalone_web_running}
            t={t}
            service="standalone"
          >
            <ServiceIntro service="standalone" t={t} />
            <section className="setupSection">
              <h3>{t("setup")}</h3>
              <StatusRow
                label={t("root")}
                ready={!!status.standalone_root_dir}
                value={status.standalone_root_dir || t("selectDirectory")}
              />
              <div className="fieldGrid">
                <label>
                  {t("port")}
                  <input
                    type="number"
                    min="1"
                    max="65535"
                    value={standalonePort}
                    onChange={(event) =>
                      setStandalonePort(Number(event.target.value || 8081))
                    }
                    disabled={status.standalone_web_running}
                  />
                </label>
                <label className="checkLine">
                  <input
                    type="checkbox"
                    checked={standaloneAllowDelete}
                    onChange={(event) =>
                      setStandaloneAllowDelete(event.target.checked)
                    }
                    disabled={status.standalone_web_running}
                  />
                  {t("allowDelete")}
                </label>
              </div>
              <WebControls
                running={status.standalone_web_running}
                url={status.standalone_web_url}
                port={standalonePort}
                setPort={setStandalonePort}
                autoOpenBrowser={standaloneAutoOpenBrowser}
                setAutoOpenBrowser={setStandaloneAutoOpenBrowser}
                busy={busy}
                t={t}
                hidePort
                onOpen={() =>
                  run(t("openStandaloneTask"), OpenStandaloneWebURL)
                }
                onCopy={() =>
                  ClipboardSetText(status.standalone_web_url).catch((err) =>
                    setError(String(err)),
                  )
                }
                onStart={() =>
                  run(t("startStandaloneTask"), () =>
                    StartFileProxyWebStandalone(standaloneOptions),
                  )
                }
                onStop={() =>
                  run(t("stopStandaloneTask"), StopFileProxyWebStandalone)
                }
                beforeStart={
                  <button
                    className="iconButton"
                    type="button"
                    disabled={!!busy || status.standalone_web_running}
                    onClick={() =>
                      run(
                        t("selectStandaloneTask"),
                        SelectStandaloneRootDirectory,
                      )
                    }
                    title={t("selectDirectory")}
                  >
                    <FolderOpen size={18} />
                  </button>
                }
                startDisabled={!status.standalone_root_dir}
              />
            </section>
          </ServicePanel>
        )}
      </section>
      <section className="logPanel">
        <div className="panelHeader">
          <h2>{t("logs")}</h2>
          <ServiceState
            running={
              status.running ||
              status.web_running ||
              status.standalone_web_running
            }
            t={t}
          />
        </div>
        <pre>{(status.logs || []).join("\n") || t("noLogs")}</pre>
      </section>
      {(error || status.last_error || busy) && (
        <div className={error || status.last_error ? "notice error" : "notice"}>
          {busy ? `${t("working")}: ${busy}` : error || status.last_error}
        </div>
      )}
    </main>
  );
}

function ServicePanel({ icon, title, running, t, service, children }) {
  return (
    <section className={`panel serviceOverview ${service}`}>
      <div className="panelHeader serviceHeader">
        <div className="panelTitle">
          {icon}
          <h2>{title}</h2>
        </div>
        <ServiceState running={running} t={t} />
      </div>
      {children}
    </section>
  );
}
function ServiceIntro({ service, t }) {
  return (
    <section className="serviceIntro">
      <p className="serviceSummary">
        {t(`${service}Summary`)} <span>{t(`${service}Flow`)}</span>
      </p>
    </section>
  );
}
function LoginCard({
  t,
  busy,
  name,
  password,
  totpCode,
  totpRequired,
  setName,
  setPassword,
  setTotpCode,
  onSubmit,
}) {
  return (
    <form
      className="loginCard"
      onSubmit={(event) => {
        event.preventDefault();
        void onSubmit();
      }}
    >
      <div className="loginPrompt">
        <LogIn size={18} />
        <span>{t("loginRequired")}</span>
      </div>
      <div className="loginFields">
        <label>
          {t("username")}
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            autoFocus={!totpRequired}
          />
        </label>
        <label>
          {t("password")}
          <input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        {totpRequired && (
          <label>
            {t("authenticatorCode")}
            <input
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength="6"
              required
              value={totpCode}
              onChange={(event) =>
                setTotpCode(event.target.value.replace(/\D/g, ""))
              }
              placeholder="123456"
              autoFocus
            />
            <small>{t("authenticatorCodeHint")}</small>
          </label>
        )}
      </div>
      <button className="primary" type="submit" disabled={!!busy}>
        <LogIn size={18} />
        {t("login")}
      </button>
    </form>
  );
}
function WebControls({
  running,
  url,
  port,
  setPort,
  autoOpenBrowser,
  setAutoOpenBrowser,
  busy,
  t,
  onOpen,
  onCopy,
  onStart,
  onStop,
  beforeStart,
  startDisabled,
  hidePort = false,
}) {
  return (
    <>
      <div className="statusRow">
        <span className={running ? "dot ready" : "dot"} />
        <span className="statusLabel">{t("browserAddress")}</span>
        <button
          className="addressLink"
          type="button"
          disabled={!running || !!busy}
          onClick={onOpen}
          title={t("openWebAddress")}
        >
          {url || t("startsOnLocalhost")}
        </button>
      </div>
      {!hidePort && (
        <label>
          {t("port")}
          <input
            type="number"
            min="1"
            max="65535"
            value={port}
            onChange={(event) => setPort(Number(event.target.value || 8080))}
            disabled={running}
          />
        </label>
      )}
      <label className="checkLine">
        <input
          type="checkbox"
          checked={autoOpenBrowser}
          onChange={(event) => setAutoOpenBrowser(event.target.checked)}
          disabled={running}
        />
        {t("autoOpenBrowser")}
      </label>
      <div className="buttonRow serviceActions">
        {beforeStart}
        <button
          className="iconButton"
          type="button"
          disabled={!running || !!busy}
          onClick={onCopy}
          title={t("copyWebAddress")}
        >
          <Copy size={18} />
        </button>
        <button
          className="primary"
          type="button"
          disabled={running || !!busy || startDisabled}
          onClick={onStart}
        >
          <Play size={18} />
          {t("start")}
        </button>
        <button type="button" disabled={!running || !!busy} onClick={onStop}>
          <Square size={18} />
          {t("stop")}
        </button>
      </div>
    </>
  );
}

function SettingsPanel({
  status,
  t,
  busy,
  apiBaseURL,
  setAPIBaseURL,
  avatarUrl,
  avatarFallback,
  displayName,
  onCancel,
  onSave,
  onLogout,
}) {
  return (
    <section className="settingsPanel simple">
      <>
        {status.logged_in && (
          <div className="settingsAccount">
            <div className="avatar small">
              {avatarUrl ? <img src={avatarUrl} alt="" /> : avatarFallback}
            </div>
            <div>
              <span>{t("account")}</span>
              <strong>{displayName}</strong>
            </div>
            <button type="button" disabled={!!busy} onClick={onLogout}>
              <LogOut size={17} />
              {t("logout")}
            </button>
          </div>
        )}
      </>
      <label>
        {t("apiDomain")}
        <input
          value={apiBaseURL}
          onChange={(event) => setAPIBaseURL(event.target.value)}
        />
      </label>
      <div className="buttonRow settingsActions">
        <button type="button" onClick={onCancel}>
          {t("cancel")}
        </button>
        <button
          className="primary"
          type="button"
          disabled={!!busy}
          onClick={onSave}
        >
          {t("save")}
        </button>
      </div>
    </section>
  );
}
function StatusRow({ label, ready, value }) {
  return (
    <div className="statusRow">
      <span className={ready ? "dot ready" : "dot"} />
      <span className="statusLabel">{label}</span>
      <span className="statusValue">{value}</span>
    </div>
  );
}
function ServiceState({ running, t }) {
  return (
    <span className={running ? "serviceState running" : "serviceState"}>
      <span />
      {running ? t("running") : t("stopped")}
    </span>
  );
}
createRoot(document.getElementById("root")).render(<App />);
