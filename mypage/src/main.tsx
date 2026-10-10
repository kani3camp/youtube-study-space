import { RouterProvider } from '@tanstack/react-router'
import { createRoot } from 'react-dom/client'
import { createApp } from './app'
import { initializeRuntime } from './firebase'
import { configuredPrivacy, type TagWindow } from './ga4-loader'
import './style.css'

const root = document.getElementById('root')
if (root)
	createRoot(root).render(
		<RouterProvider
			router={createApp(
				initializeRuntime(),
				undefined,
				configuredPrivacy(
					import.meta.env,
					window as unknown as TagWindow,
					document,
				),
			)}
		/>,
	)
