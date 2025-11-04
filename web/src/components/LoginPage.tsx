import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { useLanguage } from '../contexts/LanguageContext';

export function LoginPage() {
  const { login, isLoading } = useAuth();
  const { language } = useLanguage();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    
    if (!username.trim()) {
      setError(language === 'en' ? 'Please enter a username' : '请输入用户名');
      return;
    }
    
    if (!password.trim()) {
      setError(language === 'en' ? 'Please enter a password' : '请输入密码');
      return;
    }

    const success = await login(username, password);
    if (!success) {
      setError(language === 'en' ? 'Invalid username or password' : '用户名或密码错误');
      setPassword('');
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ background: '#0B0E11' }}>
      <div className="w-full max-w-md px-6">
        {/* Logo and Title */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 rounded-full flex items-center justify-center text-4xl mx-auto mb-4" style={{ background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)' }}>
            ⚡
          </div>
          <h1 className="text-3xl font-bold mb-2" style={{ color: '#EAECEF' }}>
            {language === 'en' ? 'AI Trading Dashboard' : 'AI交易仪表板'}
          </h1>
          <p className="text-sm" style={{ color: '#848E9C' }}>
            {language === 'en' ? 'Enter credentials to access' : '请输入用户名和密码以访问'}
          </p>
        </div>

        {/* Login Form */}
        <div className="rounded-lg p-8" style={{ background: '#1E2329', border: '1px solid #2B3139', boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)' }}>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div>
              <label htmlFor="username" className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                {language === 'en' ? 'Username' : '用户名'}
              </label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value);
                  setError('');
                }}
                disabled={isLoading}
                className="w-full px-4 py-3 rounded-lg text-base transition-all focus:outline-none focus:ring-2 focus:ring-yellow-500"
                style={{
                  background: '#0B0E11',
                  border: error ? '1px solid #F6465D' : '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                placeholder={language === 'en' ? 'Enter username...' : '请输入用户名...'}
                autoFocus
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    document.getElementById('password')?.focus();
                  }
                }}
              />
            </div>
            <div>
              <label htmlFor="password" className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                {language === 'en' ? 'Password' : '密码'}
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  setError('');
                }}
                disabled={isLoading}
                className="w-full px-4 py-3 rounded-lg text-base font-mono transition-all focus:outline-none focus:ring-2 focus:ring-yellow-500"
                style={{
                  background: '#0B0E11',
                  border: error ? '1px solid #F6465D' : '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                placeholder={language === 'en' ? 'Enter password...' : '请输入密码...'}
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    handleSubmit(e as any);
                  }
                }}
              />
              {error && (
                <p className="mt-2 text-sm flex items-center gap-1" style={{ color: '#F6465D' }}>
                  <span>⚠️</span>
                  <span>{error}</span>
                </p>
              )}
            </div>

            <button
              type="submit"
              disabled={isLoading || !username.trim() || !password.trim()}
              className="w-full py-3 rounded-lg font-semibold text-base transition-all disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90"
              style={{
                background: isLoading || !username.trim() || !password.trim() 
                  ? '#2B3139' 
                  : 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                color: isLoading || !username.trim() || !password.trim() ? '#848E9C' : '#000',
                boxShadow: isLoading || !username.trim() || !password.trim() 
                  ? 'none' 
                  : '0 4px 14px rgba(240, 185, 11, 0.4)',
              }}
            >
              {isLoading 
                ? (language === 'en' ? 'Logging in...' : '登录中...')
                : (language === 'en' ? 'Login' : '登录')
              }
            </button>
          </form>
        </div>

        {/* Footer */}
        <div className="mt-6 text-center text-xs" style={{ color: '#5E6673' }}>
          <p>{language === 'en' ? '⚠️ Trading involves risk. Use at your own discretion.' : '⚠️ 交易涉及风险，请谨慎使用。'}</p>
        </div>
      </div>
    </div>
  );
}

