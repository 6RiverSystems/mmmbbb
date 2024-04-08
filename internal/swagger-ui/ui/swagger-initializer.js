
window.onload = function() {
	window.ui = SwaggerUIBundle({
		configUrl: '../oas-ui-config',
		dom_id: "#swagger-ui",
		deepLinking: true,
		layout: "StandaloneLayout",
		displayOperationId: true,
		docExpansion: "none",
		queryConfigEnabled: true,
		showCommonExtensions: true,
		tryItOutEnabled: true,
		withCredentials: true,
		validatorUrl: null,
		presets: [
			SwaggerUIBundle.presets.apis,
			SwaggerUIStandalonePreset
		],
		plugins: [
			SwaggerUIBundle.plugins.DownloadUrl
		],
		layout: "StandaloneLayout"
	});
};
