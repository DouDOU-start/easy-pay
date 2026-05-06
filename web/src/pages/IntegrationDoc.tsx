import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import docContent from '../docs/integration.md?raw'

export default function IntegrationDoc() {
  return (
    <div className="ep-doc">
      <Markdown remarkPlugins={[remarkGfm]}>{docContent}</Markdown>
    </div>
  )
}
