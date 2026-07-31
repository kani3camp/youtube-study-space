import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'

import { router } from './router'
import './styles/global.css'

const rootElement = document.getElementById('root')

if (!rootElement) {
	throw new Error('root element is not found')
}

ReactDOM.createRoot(rootElement).render(
	<StrictMode>
		<RouterProvider router={router} />
	</StrictMode>,
)
