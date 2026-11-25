import { useEffect, useState } from 'react';
import OpenAI from 'openai';
import { supabase } from '../lib/supabase';

const openai = new OpenAI({ apiKey: import.meta.env.VITE_OPENAI_API_KEY, dangerouslyAllowBrowser: true });

type Quote = { id: string; customer: string; scope: string; total_cents: number };

export default function Quotes() {
  const [quotes, setQuotes] = useState<Quote[]>([]);
  useEffect(() => {
    supabase.from('quotes').select('*').then(({ data }) => setQuotes(data ?? []));
  }, []);

  const draft = async (q: Quote) => {
    const r = await openai.chat.completions.create({
      model: 'gpt-4o-mini',
      messages: [{ role: 'user', content: `Write a short quote letter for ${q.customer}: ${q.scope}` }],
    });
    await supabase.from('quotes').update({ letter: r.choices[0].message.content }).eq('id', q.id);
  };

  return (
    <ul>
      {quotes.map((q) => (
        <li key={q.id}>{q.customer} <button onClick={() => draft(q)}>Draft letter</button></li>
      ))}
    </ul>
  );
}
