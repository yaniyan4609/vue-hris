window.addEventListener('load', () => {
  window.ui = SwaggerUIBundle({
    url: new URL('openapi.yaml', window.location.href).pathname,
    dom_id: '#swagger-ui',
    deepLinking: true,
    filter: true,
    docExpansion: 'list',
    displayRequestDuration: true,
    withCredentials: true,
    validatorUrl: null,
    presets: [SwaggerUIBundle.presets.apis],
    layout: 'BaseLayout',
    requestInterceptor: async (request) => {
      const method = (request.method || 'GET').toUpperCase()
      const target = new URL(request.url, window.location.href)
      if (target.origin !== window.location.origin || !target.pathname.startsWith('/api/v1/') || !['POST', 'PUT', 'PATCH', 'DELETE'].includes(method) || ['/api/v1/auth/login', '/api/v1/auth/demo-login'].includes(target.pathname)) return request
      const response = await fetch('/api/v1/auth/csrf', { credentials: 'same-origin' })
      if (response.ok) {
        const payload = await response.json()
        if (payload.data?.csrf_token) request.headers['X-CSRF-Token'] = payload.data.csrf_token
      }
      return request
    },
  })
})
