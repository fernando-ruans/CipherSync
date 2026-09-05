import {useState} from 'react'
import {Tag, X} from 'lucide-react'
import {useT} from '../lib/locales'

export function TagInput({
    tags,
    onChange,
    suggestions,
}: {
    tags: string[]
    onChange: (tags: string[]) => void
    suggestions: string[]
}) {
    const t = useT()
    const [text, setText] = useState('')
    const [focused, setFocused] = useState(false)

    const available = suggestions
        .filter((s) => !tags.includes(s))
        .filter((s) => s.toLowerCase().includes(text.trim().toLowerCase()))
        .slice(0, 5)

    function addTag(raw: string) {
        const tag = raw.trim().replace(/,/g, '').toLowerCase()
        if (!tag) return
        if (!tags.includes(tag)) onChange([...tags, tag])
        setText('')
    }

    function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
        if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault()
            addTag(text)
        } else if (e.key === 'Backspace' && text === '' && tags.length > 0) {
            onChange(tags.slice(0, -1))
        }
    }

    return (
        <div>
            <span className="mb-1.5 block text-xs font-medium text-mut">{t('detail.tags')}</span>
            <div
                className="relative rounded-lg border border-edge bg-input p-2"
                onFocus={() => setFocused(true)}
                onBlur={() => setTimeout(() => setFocused(false), 150)}
            >
                <div className="flex flex-wrap items-center gap-1.5">
                    {tags.map((tag) => (
                        <span
                            key={tag}
                            className="inline-flex items-center gap-1 rounded-full bg-indigo-500/15 px-2.5 py-0.5 text-xs font-medium text-accent"
                        >
                            <Tag size={11}/>
                            {tag}
                            <button
                                type="button"
                                onClick={() => onChange(tags.filter((x) => x !== tag))}
                                className="text-accent/60 hover:text-accent"
                            >
                                <X size={11}/>
                            </button>
                        </span>
                    ))}
                    <input
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        onKeyDown={onKeyDown}
                        placeholder={tags.length === 0 ? t('detail.tagsPh') : ''}
                        className="min-w-[80px] flex-1 bg-transparent px-1 py-0.5 text-sm text-ink placeholder:text-faint outline-none"
                    />
                </div>
                {focused && text && available.length > 0 && (
                    <div className="absolute left-0 right-0 top-full z-10 mt-1 overflow-hidden rounded-lg border border-edge bg-surface shadow-xl">
                        {available.map((s) => (
                            <button
                                key={s}
                                type="button"
                                onMouseDown={() => addTag(s)}
                                className="block w-full px-3 py-1.5 text-left text-sm text-soft hover:bg-hover"
                            >
                                {s}
                            </button>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
