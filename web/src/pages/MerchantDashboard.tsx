import { useEffect, useState } from 'react'
import { Table, Tag } from 'antd'
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RTooltip, ResponsiveContainer, PieChart, Pie, Cell,
} from 'recharts'
import { merchantApi } from '../api'
import type { MerchantDashboardData as DashData } from '../api'

const statusColor: Record<string, string> = {
  pending: 'orange', paid: 'green', closed: 'default',
  refunded: 'purple', partial_refunded: 'purple', failed: 'red',
}
const statusLabel: Record<string, string> = {
  pending: '待支付', paid: '已支付', closed: '已关闭',
  refunded: '已退款', partial_refunded: '部分退款', failed: '失败',
}

const GOLD = '#d9b870'
const EMERALD = '#4ec9a0'
const CRIMSON = '#e86060'
const AZURE = '#5da7f7'

const chartTooltipStyle = {
  contentStyle: { background: '#1a1d23', border: '1px solid #2a2d33', borderRadius: 8, fontSize: 12 },
  labelStyle: { color: '#8b8d92' },
}

export default function MerchantDashboard() {
  const [data, setData] = useState<DashData | null>(null)
  useEffect(() => { merchantApi.dashboard().then(setData).catch(() => {}) }, [])

  const t = data?.today
  const o = data?.overall

  const trendData = (data?.trend ?? []).map((d) => ({
    date: d.date.slice(5),
    amount: +(d.amount / 100).toFixed(2),
    count: d.count,
  }))

  const fundData = [
    { name: '总收入', value: (o?.total_revenue ?? 0) / 100, color: EMERALD },
    { name: '总退款', value: (o?.total_refund ?? 0) / 100, color: CRIMSON },
    { name: '可结算', value: (o?.available ?? 0) / 100, color: GOLD },
  ]
  const hasFundData = fundData.some((d) => d.value > 0)

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">今日订单</div>
          <div className="value"><span className="mono">{String(t?.order_count ?? 0).padStart(2, '0')}</span></div>
          <div className="trend">● 今日</div>
        </div>
        <div className="ep-stat">
          <div className="label">今日收款</div>
          <div className="value">¥<span className="mono">{((t?.paid_amount ?? 0) / 100).toFixed(2)}</span></div>
          <div className="trend">● 人民币</div>
        </div>
        <div className="ep-stat">
          <div className="label">今日退款</div>
          <div className="value">¥<span className="mono">{((t?.refund_amount ?? 0) / 100).toFixed(2)}</span></div>
          <div className="trend dim">○ 退回</div>
        </div>
        <div className="ep-stat">
          <div className="label">可结算余额</div>
          <div className="value">¥<span className="mono">{((o?.available ?? 0) / 100).toFixed(2)}</span></div>
          <div className="trend dim">● 待结算</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 18, marginBottom: 18 }}>
        <div className="ep-panel" style={{ padding: '20px 24px' }}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 16, color: 'var(--text-primary)' }}>近 7 日收款趋势</div>
          {trendData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={trendData}>
                <defs>
                  <linearGradient id="mchGoldGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={GOLD} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={GOLD} stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e2228" />
                <XAxis dataKey="date" tick={{ fill: '#5a5d63', fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: '#5a5d63', fontSize: 11 }} axisLine={false} tickLine={false} width={50}
                  tickFormatter={(v) => `¥${v}`} />
                <RTooltip {...chartTooltipStyle}
                  formatter={(value: any, name: any) => [`¥${Number(value).toFixed(2)}`, name === 'amount' ? '金额' : '笔数']} />
                <Area type="monotone" dataKey="amount" stroke={GOLD} strokeWidth={2} fill="url(#mchGoldGrad)" name="amount" />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div style={{ height: 220, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-faint)', fontSize: 13 }}>暂无数据</div>
          )}
        </div>

        <div className="ep-panel" style={{ padding: '20px 24px', display: 'flex', flexDirection: 'column' }}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 16, color: 'var(--text-primary)' }}>资金构成</div>
          {hasFundData ? (
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
              <ResponsiveContainer width="100%" height={140}>
                <PieChart>
                  <Pie data={fundData} cx="50%" cy="50%" innerRadius={40} outerRadius={60} paddingAngle={3} dataKey="value">
                    {fundData.map((d, i) => <Cell key={i} fill={d.color} />)}
                  </Pie>
                  <RTooltip {...chartTooltipStyle} formatter={(value: any) => [`¥${Number(value).toFixed(2)}`]} />
                </PieChart>
              </ResponsiveContainer>
              <div style={{ display: 'flex', gap: 16, fontSize: 11, color: 'var(--text-secondary)', flexWrap: 'wrap', justifyContent: 'center' }}>
                {fundData.map((d) => (
                  <span key={d.name}><span style={{ color: d.color }}>●</span> {d.name} ¥{d.value.toFixed(2)}</span>
                ))}
              </div>
            </div>
          ) : (
            <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-faint)', fontSize: 13 }}>暂无数据</div>
          )}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 18, marginBottom: 18 }}>
        <div className="ep-panel" style={{ padding: '20px 24px' }}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 16, color: 'var(--text-primary)' }}>每日订单数</div>
          {trendData.length > 0 ? (
            <ResponsiveContainer width="100%" height={120}>
              <BarChart data={trendData} barSize={28}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e2228" vertical={false} />
                <XAxis dataKey="date" tick={{ fill: '#5a5d63', fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: '#5a5d63', fontSize: 11 }} axisLine={false} tickLine={false} width={30} allowDecimals={false} />
                <RTooltip {...chartTooltipStyle} formatter={(value: any) => [value, '笔数']} />
                <Bar dataKey="count" fill={AZURE} radius={[3, 3, 0, 0]} fillOpacity={0.7} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div style={{ height: 120, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-faint)', fontSize: 13 }}>暂无数据</div>
          )}
        </div>
      </div>

      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--text-primary)' }}>最近订单</div>
      <Table
        rowKey="id"
        dataSource={data?.recent_orders ?? []}
        pagination={false}
        size="small"
        scroll={{ x: 'max-content' }}
        columns={[
          { title: '平台单号', dataIndex: 'order_no', width: 220, render: (v: string) => <span className="tracked-id">{v}</span> },
          { title: '商户单号', dataIndex: 'merchant_order_no', width: 200, render: (v: string) => <span className="tracked-id">{v}</span> },
          { title: '渠道', dataIndex: 'channel', width: 90, render: (v: string) => <span className="mono" style={{ textTransform: 'uppercase', fontSize: 11 }}>{v}</span> },
          { title: '金额', dataIndex: 'amount', width: 110, render: (v: number) => <span className="money">¥{(v / 100).toFixed(2)}</span> },
          { title: '状态', dataIndex: 'status', width: 100, render: (s: string) => <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag> },
          {
            title: '时间', dataIndex: 'created_at', width: 160,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v?.slice(0, 19).replace('T', ' ')}</span>,
          },
        ]}
      />
    </>
  )
}
