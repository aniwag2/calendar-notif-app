import Image from 'next/image'
import Link from "next/link"

export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-between p-24">
      <h1>Hello</h1>
      <Link href="/calendar">Go to Calendar</Link>
    </main>
  )
}
