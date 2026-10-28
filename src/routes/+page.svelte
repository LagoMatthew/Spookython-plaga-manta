<script>
	import { onMount } from 'svelte';

	let seed = 20261028;
	let patientZeroParish = 'TARQUI';
	let dailyBudget = 500;
	let beta = 0.28;

	let simulationRunning = false;
	let canvas;
	let ctx;

	// Datos de métricas SEIR en tiempo real
	let metrics = { S: 100, E: 0, I: 0, R: 0, saved: 0, cost: 0 };

	// Lista de parroquias/sectores y sus coordenadas isométricas en la rejilla
	const parishes = [
		{ id: 'TARQUI', name: 'Tarqui', gx: 2, gy: 2, color: '#f97316' },
		{ id: 'LOS_ESTEROS', name: 'Los Esteros', gx: 5, gy: 2, color: '#06b6d4' },
		{ id: 'MANTA', name: 'Manta Centro', gx: 2, gy: 5, color: '#a855f7' },
		{ id: 'ELOY_ALFARO', name: 'Eloy Alfaro', gx: 5, gy: 5, color: '#eab308' },
		{ id: 'SAN_MATEO', name: 'San Mateo', gx: 3.5, gy: 7.5, color: '#3b82f6' }
	];

	// Agentes/Personas en movimiento
	let agents = [];

	function initAgents() {
		agents = [];
		for (let i = 0; i < 90; i++) {
			const p = parishes[Math.floor(Math.random() * parishes.length)];
			agents.push({
				x: p.gx + (Math.random() * 1.6 - 0.8),
				y: p.gy + (Math.random() * 1.6 - 0.8),
				vx: (Math.random() - 0.5) * 0.03,
				vy: (Math.random() - 0.5) * 0.03,
				state: 'S', // S: Susceptible, E: Expuesto, I: Infectado, R: Recuperado
				parish: p.id
			});
		}
	}

	// Transformación Isométrica (Grid X/Y a Pantalla Screen X/Y)
	function toIso(gx, gy, originX, originY, tileW = 48, tileH = 24) {
		const screenX = originX + (gx - gy) * (tileW / 2);
		const screenY = originY + (gx + gy) * (tileH / 2);
		return { x: screenX, y: screenY };
	}

	function renderMap() {
		if (!ctx || !canvas) return;
		ctx.clearRect(0, 0, canvas.width, canvas.height);

		const originX = canvas.width / 2;
		const originY = 80;
		const tileW = 54;
		const tileH = 27;

		// 1. DIBUJAR TERRENO ISOMÉTRICO (Grid 8x8 estilo Pixel Art)
		for (let gx = 0; gx < 8; gx++) {
			for (let gy = 0; gy < 8; gy++) {
				const pos = toIso(gx, gy, originX, originY, tileW, tileH);
				ctx.beginPath();
				ctx.moveTo(pos.x, pos.y);
				ctx.lineTo(pos.x + tileW / 2, pos.y + tileH / 2);
				ctx.lineTo(pos.x, pos.y + tileH);
				ctx.lineTo(pos.x - tileW / 2, pos.y + tileH / 2);
				ctx.closePath();

				ctx.fillStyle = (gx + gy) % 2 === 0 ? '#1e293b' : '#0f172a';
				ctx.fill();
				ctx.strokeStyle = '#334155';
				ctx.lineWidth = 1;
				ctx.stroke();
			}
		}

		// 2. DIBUJAR EDIFICIOS Y SECTORES DE MANTA
		parishes.forEach((p) => {
			const pos = toIso(p.gx, p.gy, originX, originY, tileW, tileH);

			// Sombra de los edificios
			ctx.fillStyle = 'rgba(0, 0, 0, 0.4)';
			ctx.beginPath();
			ctx.ellipse(pos.x, pos.y + 10, 25, 12, 0, 0, Math.PI * 2);
			ctx.fill();

			// Bloques Isométricos (Edificios estilo Voxel)
			const h = 28;
			// Cara Izquierda
			ctx.fillStyle = p.color;
			ctx.beginPath();
			ctx.moveTo(pos.x - 20, pos.y);
			ctx.lineTo(pos.x, pos.y + 10);
			ctx.lineTo(pos.x, pos.y + 10 - h);
			ctx.lineTo(pos.x - 20, pos.y - h);
			ctx.fill();

			// Cara Derecha
			ctx.fillStyle = '#0284c7';
			ctx.beginPath();
			ctx.moveTo(pos.x, pos.y + 10);
			ctx.lineTo(pos.x + 20, pos.y);
			ctx.lineTo(pos.x + 20, pos.y - h);
			ctx.lineTo(pos.x, pos.y + 10 - h);
			ctx.fill();

			// Techo
			ctx.fillStyle = '#38bdf8';
			ctx.beginPath();
			ctx.moveTo(pos.x, pos.y - h);
			ctx.lineTo(pos.x + 20, pos.y - 10 - h);
			ctx.lineTo(pos.x, pos.y - 20 - h);
			ctx.lineTo(pos.x - 20, pos.y - 10 - h);
			ctx.fill();

			// Etiqueta del sector
			ctx.fillStyle = '#f8fafc';
			ctx.font = 'bold 11px sans-serif';
			ctx.textAlign = 'center';
			ctx.fillText(p.name, pos.x, pos.y - h - 14);
		});

		// 3. ACTUALIZAR Y DIBUJAR AGENTES (Personas en Isométrico)
		let sCount = 0, eCount = 0, iCount = 0, rCount = 0;

		agents.forEach((ag) => {
			if (simulationRunning) {
				ag.x += ag.vx;
				ag.y += ag.vy;

				// Limites de la isla
				if (ag.x < 0.5 || ag.x > 7.5) ag.vx *= -1;
				if (ag.y < 0.5 || ag.y > 7.5) ag.vy *= -1;

				// Infección por cercanía
				if (ag.state === 'I') {
					agents.forEach((other) => {
						if (other.state === 'S') {
							const dist = Math.hypot(ag.x - other.x, ag.y - other.y);
							if (dist < 0.6 && Math.random() < beta * 0.1) {
								other.state = 'E';
								setTimeout(() => { if (other.state === 'E') other.state = 'I'; }, 1200);
							}
						}
					});
				}
			}

			if (ag.state === 'S') sCount++;
			if (ag.state === 'E') eCount++;
			if (ag.state === 'I') iCount++;
			if (ag.state === 'R') rCount++;

			const pos = toIso(ag.x, ag.y, originX, originY, tileW, tileH);

			// Sprite de persona estilo Pixel Art
			ctx.beginPath();
			ctx.arc(pos.x, pos.y - 4, 4, 0, Math.PI * 2);

			if (ag.state === 'S') ctx.fillStyle = '#10b981'; // Susceptible (Verde)
			else if (ag.state === 'E') ctx.fillStyle = '#f59e0b'; // Expuesto (Amarillo)
			else if (ag.state === 'I') ctx.fillStyle = '#ef4444'; // Infectado Zombi (Rojo)
			else ctx.fillStyle = '#3b82f6'; // Recuperado (Azul)

			ctx.fill();
			ctx.strokeStyle = '#000';
			ctx.lineWidth = 1;
			ctx.stroke();
		});

		metrics = { S: sCount, E: eCount, I: iCount, R: rCount, saved: Math.floor(rCount * 18.5), cost: simulationRunning ? metrics.cost + 5 : metrics.cost };

		requestAnimationFrame(renderMap);
	}

	function startSimulation() {
		initAgents();
		// Infectar el sector seleccionado
		agents.forEach((ag) => {
			if (ag.parish === patientZeroParish && Math.random() < 0.6) {
				ag.state = 'I';
			}
		});
		simulationRunning = true;
	}

	onMount(() => {
		ctx = canvas.getContext('2d');
		canvas.width = canvas.parentElement.clientWidth;
		canvas.height = canvas.parentElement.clientHeight;
		initAgents();
		renderMap();
	});
</script>

<div class="tactical-layout">
	<!-- PANEL LATERAL IZQUIERDO -->
	<aside class="sidebar">
		<div class="brand">
			<span class="skull">🧟‍♂️</span>
			<div>
				<h1>SPOOKYTHON</h1>
				<p>Track B: Plaga Manta Isométrica</p>
			</div>
		</div>

		<div class="control-section">
			<h3>⚙️ Parámetros del Brote</h3>

			<div class="input-group">
				<label>Semilla (Seed)</label>
				<input type="number" bind:value={seed} />
			</div>

			<div class="input-group">
				<label>Paciente Cero (Parroquia)</label>
				<select bind:value={patientZeroParish}>
					<option value="TARQUI">Tarqui</option>
					<option value="LOS_ESTEROS">Los Esteros</option>
					<option value="MANTA">Manta Centro</option>
					<option value="ELOY_ALFARO">Eloy Alfaro</option>
					<option value="SAN_MATEO">San Mateo</option>
				</select>
			</div>

			<div class="input-group">
				<label>Presupuesto Diario ($)</label>
				<input type="number" bind:value={dailyBudget} />
			</div>

			<div class="input-group">
				<label>Tasa Transmisión (β): {beta}</label>
				<input type="range" min="0.05" max="0.95" step="0.01" bind:value={beta} />
			</div>

			<button class="launch-btn" on:click={startSimulation}>
				{simulationRunning ? '🔄 REINICIAR SIMULACIÓN' : '⚡ INICIAR SIMULACIÓN ISOMÉTRICA'}
			</button>
		</div>

		<div class="legend-box">
			<p class="legend-title"> Leyenda de Agentes</p>
			<div class="legend-item"><span class="dot green"></span> Susceptibles ({metrics.S})</div>
			<div class="legend-item"><span class="dot yellow"></span> Expuestos ({metrics.E})</div>
			<div class="legend-item"><span class="dot red"></span> Infectados Zombis ({metrics.I})</div>
		</div>
	</aside>

	<!-- ÁREA CENTRAL: VISTA ISOMÉTRICA PIXEL ART -->
	<main class="main-viewport">
		<header class="viewport-header">
			<h2>🎮 Simulación Isométrica Táctica — Manta Pixel World</h2>
			<span class="step-badge">{simulationRunning ? '🚨 SIMULACIÓN EN TIEMPO REAL' : '🟢 EN ESPERA'}</span>
		</header>

		<div class="canvas-container">
			<canvas bind:this={canvas}></canvas>
		</div>

		<div class="bottom-metrics">
			<div class="metric-box">
				<span>Personas Infectadas</span>
				<h3 class="red-text">{metrics.I}</h3>
			</div>
			<div class="metric-box">
				<span>Población En Riesgo</span>
				<h3>{metrics.S}</h3>
			</div>
			<div class="metric-box">
				<span>Costo Acumulado</span>
				<h3>${metrics.cost}</h3>
			</div>
		</div>
	</main>
</div>

<style>
	:global(body) {
		background-color: #030712;
		color: #f3f4f6;
		font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
		margin: 0;
		padding: 0;
		overflow: hidden;
	}

	.tactical-layout {
		display: flex;
		height: 100vh;
		width: 100vw;
	}

	.sidebar {
		width: 330px;
		background: #0b0f19;
		border-right: 1px solid #1f2937;
		display: flex;
		flex-direction: column;
		padding: 20px;
		box-sizing: border-box;
		gap: 15px;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 10px;
		border-bottom: 1px solid #1f2937;
		padding-bottom: 12px;
	}

	.skull { font-size: 1.8rem; }
	.brand h1 { color: #f97316; font-size: 1.1rem; margin: 0; font-weight: 900; }
	.brand p { color: #9ca3af; font-size: 0.7rem; margin: 0; }

	.control-section {
		background: #111827;
		border: 1px solid #1f2937;
		border-radius: 12px;
		padding: 15px;
		flex-grow: 1;
	}

	.control-section h3 { color: #fb923c; font-size: 0.9rem; margin-top: 0; margin-bottom: 12px; }
	.input-group { margin-bottom: 10px; }
	.input-group label { display: block; font-size: 0.7rem; color: #9ca3af; margin-bottom: 3px; }
	.input-group input, .input-group select {
		width: 100%;
		background: #1f2937;
		border: 1px solid #374151;
		color: white;
		padding: 7px 10px;
		border-radius: 6px;
		font-size: 0.8rem;
		box-sizing: border-box;
	}

	.launch-btn {
		width: 100%;
		background: linear-gradient(135deg, #ea580c, #dc2626);
		color: white;
		font-weight: bold;
		padding: 10px;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-size: 0.85rem;
		margin-top: 8px;
	}

	.legend-box {
		background: #111827;
		border: 1px solid #1f2937;
		padding: 10px 14px;
		border-radius: 8px;
		font-size: 0.75rem;
	}

	.legend-title { font-weight: bold; margin: 0 0 6px 0; color: #fb923c; }
	.legend-item { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
	.dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
	.dot.green { background: #10b981; }
	.dot.yellow { background: #f59e0b; }
	.dot.red { background: #ef4444; }

	.main-viewport {
		flex-grow: 1;
		background: #030712;
		display: flex;
		flex-direction: column;
		padding: 20px;
		box-sizing: border-box;
		gap: 15px;
	}

	.viewport-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid #1f2937;
		padding-bottom: 10px;
	}

	.viewport-header h2 { color: #f3f4f6; font-size: 1.1rem; margin: 0; }
	.step-badge {
		background: rgba(249, 115, 22, 0.2);
		color: #fb923c;
		border: 1px solid rgba(249, 115, 22, 0.4);
		padding: 4px 10px;
		border-radius: 20px;
		font-size: 0.7rem;
		font-weight: bold;
	}

	.canvas-container {
		flex-grow: 1;
		background: #0b0f19;
		border: 1px solid #1f2937;
		border-radius: 16px;
		position: relative;
		overflow: hidden;
	}

	canvas {
		width: 100%;
		height: 100%;
		display: block;
	}

	.bottom-metrics {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 15px;
		height: 80px;
	}

	.metric-box {
		background: #0b0f19;
		border: 1px solid #1f2937;
		border-radius: 12px;
		padding: 10px 18px;
		display: flex;
		flex-direction: column;
		justify-content: center;
	}

	.metric-box span { font-size: 0.7rem; color: #9ca3af; text-transform: uppercase; }
	.metric-box h3 { margin: 2px 0 0 0; color: #fb923c; font-size: 1.4rem; font-family: monospace; }
	.red-text { color: #ef4444 !important; }
</style>