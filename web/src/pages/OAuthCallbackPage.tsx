import { useEffect, useState } from 'react'
import { Button, Result, Spin } from 'antd'
import { useNavigate } from 'react-router-dom'

import { setTokens } from '../api/client'

// OAuthCallbackPage 完成飞书 OAuth 登录（FR-5.3）：后端携带令牌重定向到此页，
// 令牌放在 URL fragment 中，例如：
//   {base}oauth/callback#access_token=...&refresh_token=...
// fragment 不会发送到服务器，因此令牌不会进入日志或浏览器历史
export default function OAuthCallbackPage() {
  const navigate = useNavigate()
  const [error, setError] = useState('')

  useEffect(() => {
    const params = new URLSearchParams(window.location.hash.replace(/^#/, ''))
    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    if (accessToken && refreshToken) {
      setTokens({
        access_token: accessToken,
        refresh_token: refreshToken,
        token_type: 'Bearer',
        expires_in: 0,
      })
      // 用 replace 进入应用，同时把带令牌的 fragment 从地址栏和浏览历史中抹掉
      navigate('/dashboard', { replace: true })
      return
    }
    setError('登录回调参数缺失，请重试')
  }, [navigate])

  if (error) {
    return (
      <Result
        status="error"
        title="飞书登录失败"
        subTitle={error}
        extra={
          <Button type="primary" onClick={() => navigate('/login', { replace: true })}>
            返回登录页
          </Button>
        }
      />
    )
  }
  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Spin size="large" tip="正在完成飞书登录…" />
    </div>
  )
}
