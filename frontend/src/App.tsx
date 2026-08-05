import {useEffect} from 'react'
import {Toaster} from 'react-hot-toast'
import {Loader2} from 'lucide-react'
import {useApp} from './state'
import {SetupScreen, UnlockScreen} from './components/AuthScreens'
import {MainScreen} from './components/MainScreen'
import {useAutoLock} from './lib/autolock'

function LoadingScreen() {
    return (
        <div className="flex h-full items-center justify-center">
            <Loader2 size={28} className="animate-spin text-accent"/>
        </div>
    )
}

function App() {
    const phase = useApp((s) => s.phase)
    const init = useApp((s) => s.init)
    const autolockMinutes = useApp((s) => s.autolockMinutes)

    useEffect(() => {
        void init()
    }, [init])

    useAutoLock(phase === 'main' ? autolockMinutes : 0)

    return (
        <div className="h-full">
            <Toaster
                position="top-center"
                toastOptions={{
                    style: {
                        background: 'var(--surface)',
                        color: 'var(--ink)',
                        border: '1px solid var(--edge)',
                        fontSize: '13px',
                    },
                }}
            />
            {phase === 'loading' && <LoadingScreen/>}
            {phase === 'setup' && <SetupScreen/>}
            {phase === 'unlock' && <UnlockScreen/>}
            {phase === 'main' && <MainScreen/>}
        </div>
    )
}

export default App
