import { useState } from 'react'
import { Input, InputNumber, Button, Steps, message, Alert } from 'antd'
import { CheckCircleFilled, LoadingOutlined, DatabaseOutlined, CloudServerOutlined, UserOutlined, RocketOutlined } from '@ant-design/icons'
import axios from 'axios'

const { Step } = Steps

const Field = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div style={{ marginBottom: 16 }}>
    <div style={{ marginBottom: 6, fontSize: 13, fontWeight: 500, color: 'var(--text-secondary)' }}>{label}</div>
    {children}
  </div>
)

export default function Setup() {
  const [step, setStep] = useState(0)

  // --- DB state ---
  const [dbHost, setDbHost] = useState('localhost')
  const [dbPort, setDbPort] = useState(15432)
  const [dbUser, setDbUser] = useState('easypay')
  const [dbPass, setDbPass] = useState('')
  const [dbName, setDbName] = useState('easypay')
  const [dbTested, setDbTested] = useState(false)

  // --- Redis state ---
  const [redisAddr, setRedisAddr] = useState('localhost:6379')
  const [redisPass, setRedisPass] = useState('')
  const [redisDB, setRedisDB] = useState(0)
  const [redisTested, setRedisTested] = useState(false)

  // --- Admin state ---
  const [adminEmail, setAdminEmail] = useState('')
  const [adminPwd, setAdminPwd] = useState('')
  const [adminPwdConfirm, setAdminPwdConfirm] = useState('')

  const [testing, setTesting] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [done, setDone] = useState(false)

  // --- test ---

  const testDB = async () => {
    if (!dbHost || !dbUser || !dbName) { message.error('请填写完整'); return }
    setTesting(true)
    try {
      await axios.post('/setup/test-db', { host: dbHost, port: dbPort, user: dbUser, password: dbPass, dbname: dbName })
      setDbTested(true)
      message.success('数据库连接成功')
    } catch (e: any) {
      message.error(e.response?.data?.msg || '连接失败')
      setDbTested(false)
    } finally {
      setTesting(false)
    }
  }

  const testRedis = async () => {
    setTesting(true)
    try {
      await axios.post('/setup/test-redis', { addr: redisAddr, password: redisPass, db: redisDB })
      setRedisTested(true)
      message.success('Redis 连接成功')
    } catch (e: any) {
      message.error(e.response?.data?.msg || '连接失败')
      setRedisTested(false)
    } finally {
      setTesting(false)
    }
  }

  // --- install ---

  const doInstall = async () => {
    setInstalling(true)
    try {
      await axios.post('/setup/install', {
        db: { host: dbHost, port: dbPort, user: dbUser, password: dbPass, dbname: dbName },
        redis: { addr: redisAddr, password: redisPass, db: redisDB },
        admin: { email: adminEmail, password: adminPwd },
      })
      setDone(true)
      message.success('安装成功，服务即将重启...')
      setTimeout(() => { window.location.href = '/login' }, 3000)
    } catch (e: any) {
      message.error(e.response?.data?.msg || '安装失败')
    } finally {
      setInstalling(false)
    }
  }

  // --- navigation ---

  const canNext = () => {
    if (step === 0) return dbTested
    if (step === 1) return redisTested
    if (step === 2) return adminEmail && adminPwd.length >= 6 && adminPwd === adminPwdConfirm
    return false
  }

  // --- steps ---

  const stepDB = (
    <>
      <div className="ep-setup-form-grid">
        <Field label="主机地址">
          <Input value={dbHost} onChange={e => { setDbHost(e.target.value); setDbTested(false) }} />
        </Field>
        <Field label="端口">
          <InputNumber style={{ width: '100%' }} min={1} max={65535} value={dbPort} onChange={v => { setDbPort(v ?? 15432); setDbTested(false) }} />
        </Field>
      </div>
      <div className="ep-setup-form-grid">
        <Field label="用户名">
          <Input value={dbUser} onChange={e => { setDbUser(e.target.value); setDbTested(false) }} />
        </Field>
        <Field label="密码">
          <Input.Password value={dbPass} onChange={e => { setDbPass(e.target.value); setDbTested(false) }} placeholder="数据库密码" />
        </Field>
      </div>
      <Field label="数据库名">
        <Input value={dbName} onChange={e => { setDbName(e.target.value); setDbTested(false) }} />
      </Field>
      <div className="ep-setup-test-row">
        <Button onClick={testDB} loading={testing} icon={dbTested ? <CheckCircleFilled style={{ color: 'var(--accent-emerald)' }} /> : undefined}>
          {dbTested ? '连接成功' : '测试连接'}
        </Button>
      </div>
    </>
  )

  const stepRedis = (
    <>
      <Field label="地址">
        <Input value={redisAddr} onChange={e => { setRedisAddr(e.target.value); setRedisTested(false) }} />
      </Field>
      <div className="ep-setup-form-grid">
        <Field label="密码">
          <Input.Password value={redisPass} onChange={e => { setRedisPass(e.target.value); setRedisTested(false) }} placeholder="留空表示无密码" />
        </Field>
        <Field label="数据库编号">
          <InputNumber style={{ width: '100%' }} min={0} max={15} value={redisDB} onChange={v => { setRedisDB(v ?? 0); setRedisTested(false) }} />
        </Field>
      </div>
      <div className="ep-setup-test-row">
        <Button onClick={testRedis} loading={testing} icon={redisTested ? <CheckCircleFilled style={{ color: 'var(--accent-emerald)' }} /> : undefined}>
          {redisTested ? '连接成功' : '测试连接'}
        </Button>
      </div>
    </>
  )

  const pwdError = adminPwdConfirm && adminPwd !== adminPwdConfirm ? '两次输入的密码不一致' : ''

  const stepAdmin = (
    <>
      <Field label="管理员邮箱">
        <Input value={adminEmail} onChange={e => setAdminEmail(e.target.value)} placeholder="admin@example.com" autoComplete="off" />
      </Field>
      <Field label="密码">
        <Input.Password value={adminPwd} onChange={e => setAdminPwd(e.target.value)} placeholder="至少 6 个字符" autoComplete="off" />
      </Field>
      <Field label="确认密码">
        <Input.Password
          value={adminPwdConfirm}
          onChange={e => setAdminPwdConfirm(e.target.value)}
          placeholder="再次输入密码"
          autoComplete="off"
          status={pwdError ? 'error' : undefined}
        />
        {pwdError && <div style={{ color: 'var(--accent-crimson)', fontSize: 12, marginTop: 4 }}>{pwdError}</div>}
      </Field>
    </>
  )

  const stepConfirm = (
    <div className="ep-setup-summary">
      {done ? (
        <Alert type="success" showIcon message="安装完成" description="系统正在重启，3 秒后跳转到登录页面..." />
      ) : (
        <>
          <div className="ep-setup-summary-card">
            <div className="ep-setup-summary-label">数据库</div>
            <div className="ep-setup-summary-value mono">{dbUser}@{dbHost}:{dbPort}/{dbName}</div>
          </div>
          <div className="ep-setup-summary-card">
            <div className="ep-setup-summary-label">Redis</div>
            <div className="ep-setup-summary-value mono">{redisAddr} / db{redisDB}</div>
          </div>
          <div className="ep-setup-summary-card">
            <div className="ep-setup-summary-label">管理员</div>
            <div className="ep-setup-summary-value">{adminEmail} / ••••••</div>
          </div>
          <Button type="primary" size="large" block onClick={doInstall} loading={installing} icon={<RocketOutlined />} style={{ marginTop: 16 }}>
            {installing ? '安装中...' : '开始安装'}
          </Button>
        </>
      )}
    </div>
  )

  const steps = [
    { title: '数据库', icon: <DatabaseOutlined />, content: stepDB },
    { title: 'Redis', icon: <CloudServerOutlined />, content: stepRedis },
    { title: '管理员', icon: <UserOutlined />, content: stepAdmin },
    { title: '完成', icon: <RocketOutlined />, content: stepConfirm },
  ]

  return (
    <div className="ep-setup">
      <div className="ep-setup-card">
        <div className="ep-setup-brand">
          <div className="mark">ep</div>
          <div>
            <div className="label">易支付</div>
            <div className="caption">初始化向导</div>
          </div>
        </div>

        <Steps current={step} size="small" className="ep-setup-steps">
          {steps.map((s, i) => (
            <Step key={i} title={s.title} icon={step === i && (testing || installing) ? <LoadingOutlined /> : s.icon} />
          ))}
        </Steps>

        <div className="ep-setup-body">{steps[step].content}</div>

        {step < 3 && (
          <div className="ep-setup-footer">
            {step > 0 && <Button onClick={() => setStep(step - 1)}>上一步</Button>}
            <Button type="primary" onClick={() => setStep(step + 1)} disabled={!canNext()} style={{ marginLeft: 'auto' }}>
              下一步
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
