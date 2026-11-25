import { useParams } from 'react-router-dom'
import { crews } from './schedule'

export default function Crew() {
  const { name } = useParams()
  const crew = crews.find((c) => c.name === name)
  return <ol>{crew?.jobs.map((j) => <li key={j}>{j}</li>)}</ol>
}
