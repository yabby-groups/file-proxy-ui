import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  CheckCircle2,
  FolderOpen,
  LogIn,
  Play,
  RefreshCcw,
  ShieldCheck,
  Square,
} from 'lucide-react';
import {
  EnsurePeriodicConfig,
  Login,
  PrepareCertificate,
  SelectRootDirectory,
  StartFileProxy,
  Status,
  StopFileProxy,
} from '../wailsjs/go/main/App';
import logo from './assets/logo.png';
import './styles.css';

const emptyStatus = {
  api_base_url: 'https://iot.huabot.com',
  logged_in: false,
  user_name: '',
  config: null,
  certificate: null,
  root_dir: '',
  running: false,
  last_error: '',
  logs: [],
  binary_target: '',
};

function App() {
  const [status, setStatus] = useState(emptyStatus);
  const [name, setName] = useState('');
  const [passwd, setPasswd] = useState('');
  const [thread, setThread] = useState(10);
  const [allowDelete, setAllowDelete] = useState(false);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

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

  return (
    <main className="shell">
      <header className="topbar">
        <div className="brand">
          <img src={logo} alt="" />
          <div>
            <h1>Myna File Proxy</h1>
            <p>{status.api_base_url}</p>
          </div>
        </div>
        <button className="iconButton" type="button" onClick={() => run('refresh', refresh)} disabled={!!busy}>
          <RefreshCcw size={18} />
        </button>
      </header>

      <section className="grid">
        <div className="panel loginPanel">
          <div className="panelHeader">
            <LogIn size={19} />
            <h2>Login</h2>
          </div>
          <label>
            Username
            <input value={name} onChange={(event) => setName(event.target.value)} disabled={status.logged_in || !!busy} />
          </label>
          <label>
            Password
            <input type="password" value={passwd} onChange={(event) => setPasswd(event.target.value)} disabled={status.logged_in || !!busy} />
          </label>
          <button
            className="primary"
            type="button"
            disabled={status.logged_in || !!busy}
            onClick={() => run('login', login)}
          >
            <LogIn size={18} />
            {status.logged_in ? `Logged in: ${status.user_name}` : 'Login'}
          </button>
        </div>

        <div className="panel">
          <div className="panelHeader">
            <ShieldCheck size={19} />
            <h2>Periodic</h2>
          </div>
          <StatusRow label="Config" ready={!!status.config} value={status.config?.client_name || 'Optional'} />
          <StatusRow label="Certificate" ready={!!status.certificate} value={status.certificate?.client_private_path || 'Optional'} />
          <div className="buttonRow">
            <button type="button" disabled={!status.logged_in || !!busy} onClick={() => run('config', EnsurePeriodicConfig)}>
              <CheckCircle2 size={18} />
              Config
            </button>
            <button type="button" disabled={!status.logged_in || !status.config || !!busy} onClick={() => run('certificate', PrepareCertificate)}>
              <ShieldCheck size={18} />
              Cert
            </button>
          </div>
        </div>

        <div className="panel">
          <div className="panelHeader">
            <FolderOpen size={19} />
            <h2>Worker</h2>
          </div>
          <StatusRow label="Target" ready value={status.binary_target || 'Unknown'} />
          <StatusRow label="Root" ready={!!status.root_dir} value={status.root_dir || 'Select a directory'} />
          <label>
            Threads
            <input type="number" min="1" max="128" value={thread} onChange={(event) => setThread(Number(event.target.value || 10))} />
          </label>
          <label className="checkLine">
            <input type="checkbox" checked={allowDelete} onChange={(event) => setAllowDelete(event.target.checked)} />
            Allow delete operations
          </label>
          <div className="buttonRow">
            <button type="button" disabled={!!busy || status.running} onClick={() => run('select', SelectRootDirectory)}>
              <FolderOpen size={18} />
              Select
            </button>
            <button
              className="primary"
              type="button"
              disabled={!canStart || !!busy}
              onClick={() => run('start', () => StartFileProxy({ thread, allow_delete: allowDelete }))}
            >
              <Play size={18} />
              Start
            </button>
            <button type="button" disabled={!status.running || !!busy} onClick={() => run('stop', StopFileProxy)}>
              <Square size={18} />
              Stop
            </button>
          </div>
        </div>
      </section>

      {(error || status.last_error || busy) && (
        <div className={error || status.last_error ? 'notice error' : 'notice'}>
          {busy ? `Working: ${busy}` : error || status.last_error}
        </div>
      )}

      <section className="logPanel">
        <div className="panelHeader">
          <h2>Logs</h2>
          <span className={status.running ? 'pill running' : 'pill'}>{status.running ? 'Running' : 'Stopped'}</span>
        </div>
        <pre>{(status.logs || []).join('\n') || 'No logs yet.'}</pre>
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

createRoot(document.getElementById('root')).render(<App />);
