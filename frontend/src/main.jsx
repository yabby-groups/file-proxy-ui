import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  CheckCircle2,
  Copy,
  ExternalLink,
  FolderOpen,
  Languages,
  LogIn,
  LogOut,
  Play,
  RefreshCcw,
  Settings,
  ShieldCheck,
  Square,
} from 'lucide-react';
import {
  EnsurePeriodicConfig,
  Login,
  Logout,
  OpenWebURL,
  PrepareCertificate,
  SelectRootDirectory,
  SetAPIBaseURL,
  StartFileProxy,
  StartFileProxyWeb,
  Status,
  StopFileProxy,
  StopFileProxyWeb,
} from '../wailsjs/go/main/App';
import { ClipboardSetText } from '../wailsjs/runtime/runtime';
import logo from './assets/logo.png';
import './styles.css';

const translations = {
  en: {
    settings: 'Settings', refresh: 'Refresh', apiDomain: 'API Domain', cancel: 'Cancel', save: 'Save',
    login: 'Login', username: 'Username', password: 'Password', logout: 'Log out', loggedIn: 'Logged in',
    periodic: 'Periodic', config: 'Config', certificate: 'Certificate', cert: 'Cert', optional: 'Optional',
    service: 'Service', worker: 'Worker', web: 'Client', target: 'Target', unknown: 'Unknown', root: 'Root', selectDirectory: 'Select a directory', threads: 'Threads',
    allowDelete: 'Allow delete operations', select: 'Select', start: 'Start', stop: 'Stop',
    webGateway: 'File Proxy Client', browserAddress: 'Browser address', startsOnLocalhost: 'Starts on localhost',
    port: 'Port', autoOpenBrowser: 'Open browser when ready', openWebAddress: 'Open web address',
    copyWebAddress: 'Copy web address', logs: 'Logs', running: 'Running', stopped: 'Stopped', noLogs: 'No logs yet.',
    working: 'Working', refreshTask: 'refresh', loginTask: 'login', logoutTask: 'logout', configTask: 'config',
    certTask: 'certificate', selectTask: 'select directory', startWorkerTask: 'start worker', stopWorkerTask: 'stop worker',
    startWebTask: 'start client', stopWebTask: 'stop client', openWebTask: 'open web address', domainTask: 'save domain',
  },
  zh: {
    settings: '设置', refresh: '刷新', apiDomain: 'API 域名', cancel: '取消', save: '保存',
    login: '登录', username: '用户名', password: '密码', logout: '退出登录', loggedIn: '已登录',
    periodic: '周期服务', config: '配置', certificate: '证书', cert: '证书', optional: '可选',
    service: '服务', worker: '文件服务', web: '客户端', target: '目标平台', unknown: '未知', root: '目录', selectDirectory: '选择目录', threads: '线程数',
    allowDelete: '允许删除文件', select: '选择', start: '启动', stop: '停止',
    webGateway: '文件代理客户端', browserAddress: '访问地址', startsOnLocalhost: '启动后显示本机地址',
    port: '端口', autoOpenBrowser: '启动后自动打开浏览器', openWebAddress: '打开访问地址',
    copyWebAddress: '复制访问地址', logs: '日志', running: '运行中', stopped: '已停止', noLogs: '暂无日志。',
    working: '处理中', refreshTask: '刷新', loginTask: '登录', logoutTask: '退出登录', configTask: '获取配置',
    certTask: '获取证书', selectTask: '选择目录', startWorkerTask: '启动文件服务', stopWorkerTask: '停止文件服务',
    startWebTask: '启动客户端', stopWebTask: '停止客户端', openWebTask: '打开访问地址', domainTask: '保存域名',
  },
};

const emptyStatus = {
  api_base_url: 'https://iot.huabot.com',
  logged_in: false,
  user_name: '',
  user_info: null,
  config: null,
  certificate: null,
  root_dir: '',
  start_options: null,
  running: false,
  web_running: false,
  web_url: '',
  last_error: '',
  logs: [],
  binary_target: '',
};

function App() {
  const [status, setStatus] = useState(emptyStatus);
  const [name, setName] = useState('');
  const [passwd, setPasswd] = useState('');
  const [thread, setThread] = useState(4);
  const [allowDelete, setAllowDelete] = useState(false);
  const [port, setPort] = useState(8080);
  const [autoOpenBrowser, setAutoOpenBrowser] = useState(true);
  const [language, setLanguage] = useState(() => window.localStorage.getItem('myna-file-proxy-language') || (navigator.language.startsWith('zh') ? 'zh' : 'en'));
  const [serviceView, setServiceView] = useState(() => window.localStorage.getItem('myna-file-proxy-service-view') || 'worker');
  const [apiBaseURL, setAPIBaseURL] = useState(emptyStatus.api_base_url);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const settingsLoadedRef = useRef(false);

  const refresh = useCallback(async () => {
    const next = await Status();
    setStatus(next);
  }, []);

  useEffect(() => {
    void refresh().catch((err) => setError(String(err)));
    const id = window.setInterval(() => {
      void refresh().catch(() => undefined);
    }, 1500);
    return () => window.clearInterval(id);
  }, [refresh]);

  useEffect(() => {
    if (settingsLoadedRef.current || !status.start_options) return;
    setThread(status.start_options.thread || 4);
    setAllowDelete(Boolean(status.start_options.allow_delete));
    setPort(status.start_options.port || 8080);
    setAutoOpenBrowser(status.start_options.auto_open_browser !== false);
    settingsLoadedRef.current = true;
  }, [status.start_options]);

  useEffect(() => {
    if (settingsOpen) return;
    setAPIBaseURL(status.api_base_url || emptyStatus.api_base_url);
  }, [settingsOpen, status.api_base_url]);

  const run = useCallback(async (label, fn) => {
    setBusy(label);
    setError('');
    try {
      const next = await fn();
      setStatus(next);
    } catch (err) {
      setError(err?.message || String(err));
      await refresh();
    } finally {
      setBusy('');
    }
  }, [refresh]);

  const canStart = useMemo(() => {
    return status.root_dir && !status.running;
  }, [status]);

  const login = useCallback(async () => {
    return Login(name, passwd);
  }, [name, passwd]);

  const logout = useCallback(async () => {
    setPasswd('');
    return Logout();
  }, []);

  const saveAPIBaseURL = useCallback(async () => {
    const next = await SetAPIBaseURL(apiBaseURL);
    setSettingsOpen(false);
    return next;
  }, [apiBaseURL]);

  const copyWebURL = useCallback(async () => {
    if (!status.web_url) return;
    await ClipboardSetText(status.web_url);
  }, [status.web_url]);

  const t = useCallback((key) => translations[language][key] || translations.en[key] || key, [language]);

  useEffect(() => {
    window.localStorage.setItem('myna-file-proxy-language', language);
  }, [language]);

  useEffect(() => {
    window.localStorage.setItem('myna-file-proxy-service-view', serviceView);
  }, [serviceView]);

  const displayName = status.user_info?.nick_name || status.user_info?.name || status.user_name;
  const avatarUrl = status.user_info?.avatar_url || '';
  const avatarFallback = (displayName || status.user_name || 'U').slice(0, 1).toUpperCase();

  return (
    <main className="shell">
      <header className="topbar">
        <div className="brand">
          <img src={logo} alt="" />
          <div>
            <h1>
              Myna File Proxy
              <span className="versionBadge">v1.2.0.0</span>
            </h1>
            <p>{status.api_base_url}</p>
          </div>
        </div>
        <div className="topActions">
          <div className="languageSwitch" aria-label={language === 'zh' ? '语言' : 'Language'}>
            <Languages size={16} aria-hidden="true" />
            <button className={language === 'en' ? 'active' : ''} type="button" onClick={() => setLanguage('en')} disabled={!!busy}>EN</button>
            <button className={language === 'zh' ? 'active' : ''} type="button" onClick={() => setLanguage('zh')} disabled={!!busy}>中文</button>
          </div>
          <div className="serviceSwitch" aria-label={t('service')}>
            <button className={serviceView === 'worker' ? 'active' : ''} type="button" onClick={() => setServiceView('worker')} disabled={!!busy}>{t('worker')}</button>
            <button className={serviceView === 'web' ? 'active' : ''} type="button" onClick={() => setServiceView('web')} disabled={!!busy}>{t('web')}</button>
          </div>
          <button className="iconButton" type="button" onClick={() => setSettingsOpen((open) => !open)} disabled={!!busy} title={t('settings')}>
            <Settings size={18} />
          </button>
          <button className="iconButton" type="button" onClick={() => run(t('refreshTask'), refresh)} disabled={!!busy} title={t('refresh')}>
            <RefreshCcw size={18} />
          </button>
        </div>
      </header>

      {settingsOpen && (
        <section className="settingsPanel">
          <label>
            {t('apiDomain')}
            <input value={apiBaseURL} onChange={(event) => setAPIBaseURL(event.target.value)} disabled={!!busy} />
          </label>
          <div className="buttonRow">
            <button type="button" disabled={!!busy} onClick={() => {
              setAPIBaseURL(status.api_base_url || emptyStatus.api_base_url);
              setSettingsOpen(false);
            }}>
              {t('cancel')}
            </button>
            <button className="primary" type="button" disabled={!!busy} onClick={() => run(t('domainTask'), saveAPIBaseURL)}>
              {t('save')}
            </button>
          </div>
        </section>
      )}

      <section className="grid">
        <div className="panel loginPanel">
          <div className="panelHeader">
            <LogIn size={19} />
            <h2>{t('login')}</h2>
          </div>
          {status.logged_in ? (
            <>
              <div className="userCard">
                <div className="avatar">
                  {avatarUrl ? <img src={avatarUrl} alt="" /> : avatarFallback}
                </div>
                <div className="userMeta">
                  <strong>{displayName || t('loggedIn')}</strong>
                  <span>{status.user_info?.name || status.user_name}</span>
                </div>
              </div>
              <button type="button" disabled={!!busy} onClick={() => run(t('logoutTask'), logout)}>
                <LogOut size={18} />
                {t('logout')}
              </button>
            </>
          ) : (
            <>
              <label>
                {t('username')}
                <input value={name} onChange={(event) => setName(event.target.value)} disabled={!!busy} />
              </label>
              <label>
                {t('password')}
                <input type="password" value={passwd} onChange={(event) => setPasswd(event.target.value)} disabled={!!busy} />
              </label>
              <button
                className="primary"
                type="button"
                disabled={!!busy}
                onClick={() => run(t('loginTask'), login)}
              >
                <LogIn size={18} />
                {t('login')}
              </button>
            </>
          )}
        </div>

        <div className="panel">
          <div className="panelHeader">
            <ShieldCheck size={19} />
            <h2>{t('periodic')}</h2>
          </div>
          <StatusRow label={t('config')} ready={!!status.config} value={status.config?.client_name || t('optional')} />
          <StatusRow label={t('certificate')} ready={!!status.certificate} value={status.certificate?.client_private_path || t('optional')} />
          <div className="buttonRow">
            <button type="button" disabled={!status.logged_in || !!busy} onClick={() => run(t('configTask'), EnsurePeriodicConfig)}>
              <CheckCircle2 size={18} />
              {t('config')}
            </button>
            <button type="button" disabled={!status.logged_in || !status.config || !!busy} onClick={() => run(t('certTask'), PrepareCertificate)}>
              <ShieldCheck size={18} />
              {t('cert')}
            </button>
          </div>
        </div>

        {serviceView === 'worker' && <div className="panel servicePanel">
          <div className="panelHeader serviceHeader">
            <div className="panelTitle">
              <FolderOpen size={19} />
              <h2>{t('worker')}</h2>
            </div>
            <ServiceState running={status.running} t={t} />
          </div>
          <StatusRow label={t('target')} ready value={status.binary_target || t('unknown')} />
          <StatusRow label={t('root')} ready={!!status.root_dir} value={status.root_dir || t('selectDirectory')} />
          <label>
            {t('threads')}
            <input type="number" min="1" max="16" value={thread} onChange={(event) => setThread(Number(event.target.value || 4))} />
          </label>
          <label className="checkLine">
            <input type="checkbox" checked={allowDelete} onChange={(event) => setAllowDelete(event.target.checked)} />
            {t('allowDelete')}
          </label>
          <div className="buttonRow">
            <button type="button" disabled={!!busy || status.running} onClick={() => run(t('selectTask'), SelectRootDirectory)}>
              <FolderOpen size={18} />
              {t('select')}
            </button>
            <button
              className="primary"
              type="button"
              disabled={!canStart || !!busy}
              onClick={() => run(t('startWorkerTask'), () => StartFileProxy({ thread, allow_delete: allowDelete, port, auto_open_browser: autoOpenBrowser }))}
            >
              <Play size={18} />
              {t('start')}
            </button>
            <button type="button" disabled={!status.running || !!busy} onClick={() => run(t('stopWorkerTask'), StopFileProxy)}>
              <Square size={18} />
              {t('stop')}
            </button>
          </div>
        </div>}

        {serviceView === 'web' && <div className="panel servicePanel">
          <div className="panelHeader serviceHeader">
            <div className="panelTitle">
              <ExternalLink size={19} />
              <h2>{t('webGateway')}</h2>
            </div>
            <ServiceState running={status.web_running} t={t} />
          </div>
          <div className="webAddress">
            <span className="webAddressLabel">{t('browserAddress')}</span>
            <div className="webAddressValue">
              <button className="webAddressLink" type="button" disabled={!status.web_running || !!busy} onClick={() => run(t('openWebTask'), OpenWebURL)} title={t('openWebAddress')}>
                {status.web_url || t('startsOnLocalhost')}
              </button>
              <button className="iconButton" type="button" disabled={!status.web_running || !!busy} onClick={() => copyWebURL().catch((err) => setError(String(err)))} title={t('copyWebAddress')}>
                <Copy size={17} />
              </button>
              <button className="iconButton" type="button" disabled={!status.web_running || !!busy} onClick={() => run(t('openWebTask'), OpenWebURL)} title={t('openWebAddress')}>
                <ExternalLink size={17} />
              </button>
            </div>
          </div>
          <label>
            {t('port')}
            <input type="number" min="1" max="65535" value={port} onChange={(event) => setPort(Number(event.target.value || 8080))} disabled={!!busy || status.web_running} />
          </label>
          <label className="checkLine">
            <input type="checkbox" checked={autoOpenBrowser} onChange={(event) => setAutoOpenBrowser(event.target.checked)} disabled={!!busy || status.web_running} />
            {t('autoOpenBrowser')}
          </label>
          <div className="buttonRow">
            <button className="primary" type="button" disabled={status.web_running || !!busy} onClick={() => run(t('startWebTask'), () => StartFileProxyWeb({ thread, allow_delete: allowDelete, port, auto_open_browser: autoOpenBrowser }))}>
              <Play size={18} />
              {t('start')}
            </button>
            <button type="button" disabled={!status.web_running || !!busy} onClick={() => run(t('stopWebTask'), StopFileProxyWeb)}>
              <Square size={18} />
              {t('stop')}
            </button>
          </div>
        </div>}
      </section>

      {(error || status.last_error || busy) && (
        <div className={error || status.last_error ? 'notice error' : 'notice'}>
          {busy ? `${t('working')}: ${busy}` : error || status.last_error}
        </div>
      )}

      <section className="logPanel">
        <div className="panelHeader">
          <h2>{t('logs')}</h2>
          <span className={status.running || status.web_running ? 'pill running' : 'pill'}>{status.running || status.web_running ? t('running') : t('stopped')}</span>
        </div>
        <pre>{(status.logs || []).join('\n') || t('noLogs')}</pre>
      </section>
    </main>
  );
}

function StatusRow({ label, ready, value }) {
  return (
    <div className="statusRow">
      <span className={ready ? 'dot ready' : 'dot'} />
      <span className="statusLabel">{label}</span>
      <span className="statusValue">{value}</span>
    </div>
  );
}

function ServiceState({ running, t }) {
  return (
    <span className={running ? 'serviceState running' : 'serviceState'}>
      <span />
      {running ? t('running') : t('stopped')}
    </span>
  );
}

createRoot(document.getElementById('root')).render(<App />);
