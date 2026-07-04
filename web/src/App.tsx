import React, { useState } from 'react';

interface ShortenResponse {
  short_code: string;
  short_url: string;
}

export default function App() {
  // Auth State
  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [isLogin, setIsLogin] = useState(true);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [authFeedback, setAuthFeedback] = useState({ type: '', message: '' });

  // App State
  const [longUrl, setLongUrl] = useState('');
  const [result, setResult] = useState<ShortenResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [activeToggle, setActiveToggle] = useState('Custom slug');

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthFeedback({ type: '', message: '' });
    setLoading(true);

    const endpoint = isLogin ? '/login' : '/register';
    
    try {
      const response = await fetch(`http://localhost:8080${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'Authentication failed');
      }

      const data = await response.json();

      if (isLogin) {
        setToken(data.token);
        localStorage.setItem('token', data.token);
      } else {
        setIsLogin(true);
        setAuthFeedback({ type: 'success', message: 'Registration successful! Please log in.' });
        setPassword('');
      }
    } catch (err: any) {
      setAuthFeedback({ type: 'error', message: err.message });
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    setToken('');
    localStorage.removeItem('token');
    setResult(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!longUrl) return;

    setLoading(true);
    setResult(null);

    try {
      const response = await fetch('http://localhost:8080/shorten', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}` // Pass token to protected routes
        },
        body: JSON.stringify({ url: longUrl }),
      });

      if (!response.ok) throw new Error('Failed to shorten URL');
      const data: ShortenResponse = await response.json();
      setResult(data);
      setLongUrl('');
    } catch (err: any) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const toggles = ['Custom slug', 'Expiry', 'QR code'];

  return (
    <div className="min-h-screen bg-sage-bg p-6 md:p-12 flex flex-col items-center">
      
      {/* Top Nav Bar */}
      <header className="w-full max-w-5xl h-24 rounded-[20px] shadow-neu-raised flex items-center justify-between px-8 mb-16 md:mb-24">
        <div>
          <h1 className="font-fraunces text-2xl md:text-3xl font-semibold tracking-[-0.03em] text-sage-text">
            Ledger Link Engine
          </h1>
          <p className="font-mono text-[10px] md:text-xs uppercase tracking-widest text-sage-text/60 mt-1">
            Distributed URL Shortener Core
          </p>
        </div>
        <div className="hidden md:flex gap-5 items-center">
          <a href="http://localhost:3001" target="_blank" rel="noreferrer" className="shadow-neu-raised-sm px-5 py-2.5 rounded-full hover:shadow-neu-pressed-sm transition-shadow duration-300">
            <span className="font-mono text-xs text-sage-text/80">Grafana Metrics</span>
          </a>
          <a href="http://localhost:15672" target="_blank" rel="noreferrer" className="shadow-neu-raised-sm px-5 py-2.5 rounded-full hover:shadow-neu-pressed-sm transition-shadow duration-300">
            <span className="font-mono text-xs text-sage-text/80">RabbitMQ Queue</span>
          </a>
          {token && (
            <button onClick={handleLogout} className="ml-4 shadow-neu-raised-sm px-5 py-2.5 rounded-full hover:shadow-neu-pressed-sm transition-shadow duration-300">
              <span className="font-mono text-xs font-bold text-sage-accent">Logout</span>
            </button>
          )}
        </div>
      </header>

      {/* Main Card */}
      <main className="w-full max-w-2xl rounded-[32px] shadow-neu-raised p-8 md:p-14 flex flex-col items-center">
        
        {!token ? (
          // Authentication View
          <div className="w-full flex flex-col items-center">
             <h2 className="font-fraunces text-3xl md:text-4xl font-semibold tracking-[-0.04em] mb-2 text-sage-text">
              {isLogin ? 'Welcome Back' : 'Create Account'}
            </h2>
            <p className="font-manrope text-sm text-sage-text/60 mb-10">
              {isLogin ? 'Authenticate to access the engine' : 'Register to secure your links'}
            </p>

            <form onSubmit={handleAuth} className="w-full flex flex-col items-center">
              <div className="w-full mb-6">
                <input
                  type="email"
                  required
                  placeholder="name@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full h-16 rounded-2xl shadow-neu-pressed bg-sage-bg px-6 font-mono text-sm md:text-base outline-none placeholder:text-sage-text/40 text-sage-text"
                />
              </div>
              <div className="w-full mb-8">
                <input
                  type="password"
                  required
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full h-16 rounded-2xl shadow-neu-pressed bg-sage-bg px-6 font-mono text-sm md:text-base outline-none placeholder:text-sage-text/40 text-sage-text"
                />
              </div>

              {authFeedback.message && (
                <div className={`mb-6 text-sm font-manrope font-semibold ${authFeedback.type === 'error' ? 'text-red-500/80' : 'text-sage-accent'}`}>
                  {authFeedback.message}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full h-16 rounded-2xl shadow-neu-raised active:shadow-neu-pressed font-manrope font-bold text-lg text-sage-accent transition-shadow duration-200 mb-8 disabled:opacity-50"
              >
                {loading ? 'Processing...' : (isLogin ? 'Log In' : 'Register')}
              </button>
            </form>

            <button
              onClick={() => {
                setIsLogin(!isLogin);
                setAuthFeedback({ type: '', message: '' });
              }}
              className="font-manrope text-sm font-semibold text-sage-text/60 hover:text-sage-accent transition-colors"
            >
              {isLogin ? "Don't have an account? Register" : "Already have an account? Log in"}
            </button>
          </div>
        ) : (
          // Application View
          <>
            <h2 className="font-fraunces text-3xl md:text-4xl font-semibold tracking-[-0.04em] mb-12 text-sage-text">
              Shorten a Long URL
            </h2>

            <form onSubmit={handleSubmit} className="w-full flex flex-col items-center">
              <div className="w-full mb-8">
                <input
                  type="url"
                  required
                  placeholder="https://Long URL"
                  value={longUrl}
                  onChange={(e) => setLongUrl(e.target.value)}
                  className="w-full h-16 rounded-2xl shadow-neu-pressed bg-sage-bg px-6 font-mono text-sm md:text-base outline-none placeholder:text-sage-text/40 text-sage-text"
                />
              </div>

              <button
                type="submit"
                disabled={loading}
                className="w-full h-16 rounded-2xl shadow-neu-raised active:shadow-neu-pressed font-manrope font-bold text-lg text-sage-accent transition-shadow duration-200 mb-10 disabled:opacity-50"
              >
                {loading ? 'Compressing...' : 'Compress'}
              </button>
            </form>

            <div className="flex flex-wrap justify-center gap-4">
              {toggles.map((pill) => (
                <button
                  key={pill}
                  type="button"
                  onClick={() => setActiveToggle(pill)}
                  className={`px-6 py-2.5 rounded-full text-sm font-manrope font-semibold transition-all duration-300 ${
                    activeToggle === pill
                      ? 'shadow-neu-pressed-sm text-sage-accent'
                      : 'shadow-neu-raised-sm text-sage-text/60'
                  }`}
                >
                  {pill}
                </button>
              ))}
            </div>

            {result && (
              <div className="mt-12 p-6 w-full rounded-2xl shadow-neu-pressed-sm flex flex-col items-center text-center animate-pulse">
                <span className="font-mono text-xs text-sage-text/60 mb-2 uppercase tracking-wider">Result Generated</span>
                <a href={result.short_url} target="_blank" rel="noreferrer" className="font-manrope font-bold text-sage-accent text-xl hover:opacity-80 transition-opacity">
                  {result.short_url}
                </a>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
}