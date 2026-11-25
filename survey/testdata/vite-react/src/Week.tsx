import { Link } from 'react-router-dom'
import { crews } from './schedule'

export default function Week() {
  return (
    <ul>
      {crews.map((c) => (
        <li key={c.name}>
          <Link to={`/crew/${c.name}`}>{c.name}</Link>: {c.jobs.length} jobs
        </li>
      ))}
    </ul>
  )
}
