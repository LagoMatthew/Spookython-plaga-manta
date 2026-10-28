<script>
	import { onMount, onDestroy } from 'svelte';

	let apiUrl = $state('http://localhost:8080/simulate');
	let seed = $state(20261028);
	let patientZeroParish = $state('TARQUI');
	let dailyBudget = $state(120);
	let beta = $state(0.32);
	let vaccineEffectiveness = $state(0.85);
	let runs = $state(100);

	let simulationRunning = $state(false);
	let loadingBackend = $state(false);
	let currentDay = $state(0);
	let backendData = $state(null);
	let viewMode = $state('smartContainment'); // 'smartContainment' | 'randomIntervention' | 'noIntervention'
	let dayTimer = null;

	let canvas;
	let ctx;
	let animFrame;

	let metrics = $state({ S: 2500, E: 0, I: 0, R: 0, savedVsNone: 0, savedVsRandom: 0, peakRed: 0, cost: 0 });

	// Coordenadas más separadas para que no se monten los textos en el mapa isométrico
	const parishes = [
		{ id: 'TARQUI', name: 'Tarqui (Playita Mía)', gx: 1.5, gy: 1.5, color: '#f97316' },
		{ id: 'LOS_ESTEROS', name: 'Los Esteros (Buses)', gx: 6.0, gy: 1.5, color: '#06b6d4' },
		{ id: 'MANTA_CENTRO', name: 'Manta Centro', gx: 1.5, gy: 6.0, color: '#a855f7' },
		{ id: 'ELOY_ALFARO', name: 'Eloy Alfaro (ULEAM)', gx: 6.0, gy: 6.0, color: '#eab308' },
		{ id: 'SAN_MATEO', name: 'San Mateo', gx: 3.8, gy: 3.8, color: '#3b82f6' }
	];

	let agents = [];

	function initAgents() {
		agents = [];
		for (let i = 0; i < 150; i++) {
			const p = parishes[i % parishes.length];
			agents.push({
				x: p.gx + (Math.random() * 1.4 - 0.7),
				y: p.gy + (Math.random() * 1.4 - 0.7),
				baseX: p.gx,
				baseY: p.gy,
				vx: (Math.random() - 0.5) * 0.025,
				vy: (Math.random() - 0.5) * 0.025,
				state: 'S',
				parish: p.id
			});
		}
	}

	function syncAgentsWithDay(dayIdx) {
		if (!backendData?.strategies) return;
		const stratData = backendData.strategies[viewMode];
		if (!stratData?.dailyCurve) return;

		const curvePoint = stratData.dailyCurve[dayIdx] || stratData.dailyCurve[0];
		const totalPop = (curvePoint.S + curvePoint.E + curvePoint.I + curvePoint.R) || 2500;

		// Si estamos viendo smartContainment y hay desglose por parroquia, usarlo; si no, usar proporción global
		const breakdown = backendData.strategies.smartContainment?.parishBreakdownByDay?.[dayIdx]?.parishes;

		parishes.forEach((p) => {
			const pAgents = agents.filter((a) => a.parish === p.id);
			let nI, nE, nR;

			if (viewMode === 'smartContainment' && breakdown && breakdown[p.id]) {
				const counts = breakdown[p.id];
				const pTot = (counts.S + counts.E + counts.I + counts.R) || 1;
				nI = Math.round((counts.I / pTot) * pAgents.length);
				nE = Math.round((counts.E / pTot) * pAgents.length);
				nR = Math.round((counts.R / pTot) * pAgents.length);
			} else {
				nI = Math.round((curvePoint.I / totalPop) * pAgents.length);
				nE = Math.round((curvePoint.E / totalPop) * pAgents.length);
				nR = Math.round((curvePoint.R / totalPop) * pAgents.length);
			}

			pAgents.forEach((ag, idx) => {
				if (idx < nI) ag.state = 'I';
				else if (idx < nI + nE) ag.state = 'E';
				else if (idx < nI + nE + nR) ag.state = 'R';
				else ag.state = 'S';
			});
		});

		metrics = {
			S: curvePoint.S,
			E: curvePoint.E,
			I: curvePoint.I,
			R: curvePoint.R,
			savedVsNone: backendData.summary.savedVsNone,
			savedVsRandom: backendData.summary.savedVsRandom,
			peakRed: backendData.summary.peakReductionPct,
			cost: backendData.summary.totalCostSpent
		};
	}

	function toIso(gx, gy, originX, originY, tileW = 46, tileH = 23) {
		return {
			x: originX + (gx - gy) * (tileW / 2),
			y: originY + (gx + gy) * (tileH / 2)
		};
	}

	function renderMap() {
		if (!ctx || !canvas) return;
		if (canvas.width !== canvas.parentElement.clientWidth || canvas.height !== canvas.parentElement.clientHeight) {
			canvas.width = canvas.parentElement.clientWidth;
			canvas.height = canvas.parentElement.clientHeight;
		}
		ctx.clearRect(0, 0, canvas.width, canvas.height);

		const originX = canvas.width / 2;
		const originY = 45;
		const tileW = 46;
		const tileH = 23;

		for (let gx = 0; gx < 8; gx++) {
			for (let gy = 0; gy < 8; gy++) {
				const pos = toIso(gx, gy, originX, originY, tileW, tileH);
				ctx.beginPath();
				ctx.moveTo(pos.x, pos.y);
				ctx.lineTo(pos.x + tileW / 2, pos.y + tileH / 2);
				ctx.lineTo(pos.x, pos.y + tileH);
				ctx.lineTo(pos.x - tileW / 2, pos.y + tileH / 2);
				ctx.closePath();
				ctx.fillStyle = (gx + gy) % 2 === 0 ? '#111827' : '#0b0f19';
				ctx.fill();
				ctx.strokeStyle = '#1e293b';
				ctx.stroke();
			}
		}

		parishes.forEach((p) => {
			const pos = toIso(p.gx, p.gy, originX, originY, tileW, tileH);
			const isZero = p.id === patientZeroParish;

			ctx.fillStyle = isZero ? 'rgba(239, 68, 68, 0.35)' : 'rgba(0, 0, 0, 0.45)';
			ctx.beginPath();
			ctx.ellipse(pos.x, pos.y + 8, 26, 13, 0, 0, Math.PI * 2);
			ctx.fill();

			const h = isZero ? 30 : 22;
			ctx.fillStyle = p.color;
			ctx.fillRect(pos.x - 14, pos.y - h, 28, h);

			ctx.fillStyle = isZero ? '#fca5a5' : '#f8fafc';
			ctx.font = 'bold 11px sans-serif';
			ctx.textAlign = 'center';
			ctx.fillText(p.name, pos.x, pos.y - h - 8);
		});

		agents.forEach((ag) => {
			if (simulationRunning) {
				ag.x += ag.vx;
				ag.y += ag.vy;
				if (Math.abs(ag.x - ag.baseX) > 1.1) ag.vx *= -1;
				if (Math.abs(ag.y - ag.baseY) > 1.1) ag.vy *= -1;
			}
			const pos = toIso(ag.x, ag.y, originX, originY, tileW, tileH);
			ctx.beginPath();
			ctx.arc(pos.x, pos.y - 3, ag.state === 'I' ? 4.8 : 3.6, 0, Math.PI * 2);
			if (ag.state === 'S') ctx.fillStyle = '#10b981';
			else if (ag.state === 'E') ctx.fillStyle = '#f59e0b';
			else if (ag.state === 'I') ctx.fillStyle = '#ef4444';
			else ctx.fillStyle = '#3b82f6';
			ctx.fill();
			ctx.strokeStyle = '#000';
			ctx.lineWidth = 1;
			ctx.stroke();
		});

		animFrame = requestAnimationFrame(renderMap);
	}

	function togglePlay() {
		if (simulationRunning) {
			simulationRunning = false;
			if (dayTimer) clearInterval(dayTimer);
		} else {
			if (currentDay >= 60) currentDay = 0;
			simulationRunning = true;
			dayTimer = setInterval(() => {
				if (currentDay < 60) {
					currentDay++;
					syncAgentsWithDay(currentDay);
				} else {
					simulationRunning = false;
					clearInterval(dayTimer);
				}
			}, 200);
		}
	}

	async function startSimulation() {
		loadingBackend = true;
		simulationRunning = false;
		if (dayTimer) clearInterval(dayTimer);

		try {
			const response = await fetch(apiUrl, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					seed: Number(seed),
					runs: Number(runs),
					days: 60,
					populationSize: 2500,
					patientZeroParish: patientZeroParish,
					dailyBudget: Number(dailyBudget),
					beta: Number(beta),
					vaccineEffectiveness: Number(vaccineEffectiveness)
				})
			});

			if (response.ok) {
				backendData = await response.json();
				currentDay = 0;
				syncAgentsWithDay(0);
				togglePlay();
			}
		} catch (error) {
			alert('Verifica que Go esté encendido en otra terminal: go run ./cmd/local/main.go');
		} finally {
			loadingBackend = false;
		}
	}

	function changeViewMode(mode) {
		viewMode = mode;
		syncAgentsWithDay(currentDay);
	}

	function applySurprise(preset) {
		if (preset === 'DOUBLE') {
			beta = 0.58; dailyBudget = 80; vaccineEffectiveness = 0.85;
		} else if (preset === 'HALF_VAX') {
			beta = 0.38; vaccineEffectiveness = 0.45; dailyBudget = 120;
		} else if (preset === 'MARKET') {
			patientZeroParish = 'TARQUI'; beta = 0.44; dailyBudget = 100;
		}
		startSimulation();
	}

	function buildCurvePoints(curve, maxVal) {
		if (!curve || curve.length === 0) return '';
		return curve.map((pt, idx) => {
			const x = (idx / (curve.length - 1)) * 520 + 30;
			const y = 135 - (pt.I / maxVal) * 115;
			return `${x.toFixed(1)},${y.toFixed(1)}`;
		}).join(' ');
	}

	onMount(() => {
		ctx = canvas.getContext('2d');
		initAgents();
		renderMap();
		// NO ejecutamos startSimulation() solo; esperamos a que tú des clic
	});

	onDestroy(() => {
		if (dayTimer) clearInterval(dayTimer);
		if (animFrame) cancelAnimationFrame(animFrame);
	});
</script>

<div class="tactical-layout">
	<!-- BARRA LATERAL IZQUIERDA CON ANCHO FIJO (YA NO SE APLASTA) -->
	<aside class="sidebar">
		<div class="brand">
			<span class="skull">🧟‍♂️</span>
			<div>
				<h1>SPOOKYTHON 2026 · MANTA</h1>
				<p>Track B: Comité de Contención SEIR</p>
			</div>
		</div>

		<div class="control-section">
			<h3>⚙️ Controles del Simulador</h3>

			<div class="input-group">
				<label>Semilla Reproducible (Seed Jurado)</label>
				<input type="number" bind:value={seed} />
			</div>

			<div class="input-group">
				<label>Paciente Cero (Parroquia Manta)</label>
				<select bind:value={patientZeroParish}>
					<option value="TARQUI">Tarqui (Mercado Playita Mía)</option>
					<option value="LOS_ESTEROS">Los Esteros (Rutas de Buses)</option>
					<option value="MANTA_CENTRO">Manta Centro (Zona Comercial)</option>
					<option value="ELOY_ALFARO">Eloy Alfaro (Campus ULEAM)</option>
					<option value="SAN_MATEO">San Mateo</option>
				</select>
			</div>

			<div class="input-group">
				<label>Presupuesto Diario ($): <b>${dailyBudget}/día</b></label>
				<input type="range" min="20" max="600" step="20" bind:value={dailyBudget} />
			</div>

			<div class="input-group">
				<label>Tasa de Contagio (β): <b>{beta}</b></label>
				<input type="range" min="0.10" max="0.85" step="0.02" bind:value={beta} />
			</div>

			<div class="input-group">
				<label>Efectividad Vacuna: <b>{Math.round(vaccineEffectiveness * 100)}%</b></label>
				<input type="range" min="0.20" max="1.0" step="0.05" bind:value={vaccineEffectiveness} />
			</div>

			<button class="launch-btn" onclick={startSimulation}>
				{loadingBackend ? '⏳ CALCULANDO...' : '🚀 EJECUTAR SIMULACIÓN'}
			</button>

			<div class="surprise-box">
				<p class="legend-title">🎃 Prueba de Fuego: Brote Sorpresa</p>
				<div class="surprise-grid">
					<button class="mini-btn" onclick={() => applySurprise('DOUBLE')}>🔥 Doble β / Mitad $</button>
					<button class="mini-btn" onclick={() => applySurprise('HALF_VAX')}>🧬 Vacuna al 45%</button>
					<button class="mini-btn" onclick={() => applySurprise('MARKET')}>🛒 Brote en Tarqui</button>
				</div>
			</div>
		</div>

		<div class="legend-box">
			<p class="legend-title">Población en Día {currentDay}/60 ({viewMode === 'smartContainment' ? 'Nuestra Estrategia' : viewMode === 'randomIntervention' ? 'Al Azar' : 'Sin Intervenir'})</p>
			<div class="legend-item"><span class="dot green"></span> 🟢 Sanos Susceptibles: <b>{metrics.S}</b></div>
			<div class="legend-item"><span class="dot yellow"></span> 🟡 Mordidos (Incubando): <b>{metrics.E}</b></div>
			<div class="legend-item"><span class="dot red"></span> 🔴 Zombies Activos: <b>{metrics.I}</b></div>
			<div class="legend-item"><span class="dot blue"></span> 🔵 Exorcizados / Inmunes: <b>{metrics.R}</b></div>
		</div>
	</aside>

	<!-- PANEL PRINCIPAL -->
	<main class="main-viewport">
		<header class="viewport-header">
			<div style="display:flex; align-items:center; gap:10px;">
				<h2>🎮 Mapa Táctico de Manta (Día {currentDay} de 60)</h2>
				{#if backendData}
					<button class="play-btn" onclick={togglePlay}>
						{simulationRunning ? '⏸️ Pausar' : '▶️ Reproducir'}
					</button>
				{/if}
			</div>

			<div style="display:flex; gap:8px; align-items:center;">
				<span style="font-size:0.75rem; color:#94a3b8;">Ver en mapa:</span>
				<button class="mode-btn {viewMode === 'smartContainment' ? 'active-green' : ''}" onclick={() => changeViewMode('smartContainment')}>🟢 Nuestra Estrategia</button>
				<button class="mode-btn {viewMode === 'randomIntervention' ? 'active-yellow' : ''}" onclick={() => changeViewMode('randomIntervention')}>🟡 Al Azar</button>
				<button class="mode-btn {viewMode === 'noIntervention' ? 'active-red' : ''}" onclick={() => changeViewMode('noIntervention')}>🔴 No Intervenir</button>
				<input type="range" min="0" max="60" bind:value={currentDay} oninput={() => syncAgentsWithDay(currentDay)} style="width:120px;" />
			</div>
		</header>

		<div class="bottom-metrics">
			<div class="metric-box">
				<span>Vidas Salvadas vs No Intervenir</span>
				<h3 class="green-text">+{metrics.savedVsNone}</h3>
			</div>
			<div class="metric-box">
				<span>Vidas Salvadas vs Actuar al Azar</span>
				<h3 class="cyan-text">+{metrics.savedVsRandom}</h3>
			</div>
			<div class="metric-box">
				<span>Reducción del Pico Zombie</span>
				<h3>{metrics.peakRed}%</h3>
			</div>
			<div class="metric-box">
				<span>Presupuesto Total Invertido</span>
				<h3>${metrics.cost}</h3>
			</div>
		</div>

		<div class="split-middle">
			<div class="canvas-container">
				<canvas bind:this={canvas}></canvas>
			</div>

			<div class="chart-panel">
				<h3>📈 Comparativa de Contagios ({runs} Corridas Monte Carlo)</h3>
				{#if backendData}
					{@const maxPeak = Math.max(backendData.strategies.noIntervention.peakActiveInfected, 50)}
					<div class="strategy-tags">
						<span style="color:#ef4444;">━━ No Intervenir ({backendData.strategies.noIntervention.totalInfectedAvg} inf.)</span>
						<span style="color:#eab308;">━━ Al Azar ({backendData.strategies.randomIntervention.totalInfectedAvg} inf.)</span>
						<span style="color:#10b981;">━━ Nuestra Estrategia ({backendData.strategies.smartContainment.totalInfectedAvg} inf.)</span>
					</div>
					<svg viewBox="0 0 560 150" class="curve-svg">
						<line x1="30" y1="135" x2="550" y2="135" stroke="#334155" />
						<line x1="30" y1="15" x2="30" y2="135" stroke="#334155" />
						<polyline fill="none" stroke="#ef4444" stroke-width="2.5" points={buildCurvePoints(backendData.strategies.noIntervention.dailyCurve, maxPeak)} />
						<polyline fill="none" stroke="#eab308" stroke-width="2.5" stroke-dasharray="4,4" points={buildCurvePoints(backendData.strategies.randomIntervention.dailyCurve, maxPeak)} />
						<polyline fill="none" stroke="#10b981" stroke-width="3.5" points={buildCurvePoints(backendData.strategies.smartContainment.dailyCurve, maxPeak)} />
					</svg>
				{:else}
					<div style="height:150px; display:flex; align-items:center; justify-content:center; color:#64748b; font-size:0.85rem; border:1px dashed #334155; border-radius:8px;">
						Presiona "🚀 EJECUTAR SIMULACIÓN" a la izquierda para correr el motor Monte Carlo.
					</div>
				{/if}

				{#if backendData?.summary?.mayorReport}
					<div class="mayor-box">
						<h4>📜 Informe Ejecutivo para el Alcalde de Manta</h4>
						<p>{backendData.summary.mayorReport}</p>
					</div>
				{/if}
			</div>
		</div>
	</main>
</div>

<style>
	:global(body) {
		background-color: #030712; color: #f3f4f6;
		font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
		margin: 0; padding: 0; overflow: hidden;
	}
	.tactical-layout { display: flex; height: 100vh; width: 100vw; overflow: hidden; }
	.sidebar {
		width: 320px; min-width: 320px; flex-shrink: 0;
		background: #0b0f19; border-right: 1px solid #1f2937;
		display: flex; flex-direction: column; padding: 14px; box-sizing: border-box; gap: 10px; overflow-y: auto;
	}
	.brand { display: flex; align-items: center; gap: 10px; border-bottom: 1px solid #1f2937; padding-bottom: 8px; }
	.skull { font-size: 1.7rem; }
	.brand h1 { color: #f97316; font-size: 0.95rem; margin: 0; font-weight: 900; }
	.brand p { color: #9ca3af; font-size: 0.7rem; margin: 0; }
	.control-section { background: #111827; border: 1px solid #1f2937; border-radius: 10px; padding: 12px; }
	.control-section h3 { color: #fb923c; font-size: 0.85rem; margin: 0 0 8px 0; }
	.input-group { margin-bottom: 8px; }
	.input-group label { display: block; font-size: 0.72rem; color: #cbd5e1; margin-bottom: 3px; }
	.input-group input, .input-group select {
		width: 100%; background: #1f2937; border: 1px solid #374151; color: white;
		padding: 6px 8px; border-radius: 6px; font-size: 0.8rem; box-sizing: border-box; margin-top: 0;
	}
	.launch-btn {
		width: 100%; background: linear-gradient(135deg, #10b981, #059669); color: #022c22;
		font-weight: 900; padding: 10px; border: none; border-radius: 8px; cursor: pointer; font-size: 0.82rem; margin-top: 6px;
	}
	.surprise-box { margin-top: 10px; border-top: 1px solid #1f2937; padding-top: 8px; }
	.surprise-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 5px; }
	.mini-btn {
		background: #1e293b; color: #fbbf24; border: 1px solid #475569; padding: 6px 4px;
		font-size: 0.68rem; border-radius: 6px; cursor: pointer; margin-top: 0; width: 100%;
	}
	.play-btn {
		background: #2563eb; color: white; border: none; padding: 5px 12px; border-radius: 6px;
		font-size: 0.75rem; cursor: pointer; width: auto; margin-top: 0;
	}
	.mode-btn {
		background: #1e293b; color: #cbd5e1; border: 1px solid #334155; padding: 5px 10px;
		border-radius: 6px; font-size: 0.72rem; cursor: pointer; width: auto; margin-top: 0;
	}
	.active-green { background: #065f46; color: #6ee7b7; border-color: #10b981; }
	.active-yellow { background: #713f12; color: #fde047; border-color: #eab308; }
	.active-red { background: #7f1d1d; color: #fca5a5; border-color: #ef4444; }
	.legend-box { background: #111827; border: 1px solid #1f2937; padding: 10px 12px; border-radius: 8px; font-size: 0.74rem; }
	.legend-title { font-weight: bold; margin: 0 0 5px 0; color: #fb923c; font-size: 0.75rem; }
	.legend-item { display: flex; align-items: center; gap: 8px; margin-bottom: 3px; }
	.dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
	.dot.green { background: #10b981; }
	.dot.yellow { background: #f59e0b; }
	.dot.red { background: #ef4444; }
	.dot.blue { background: #3b82f6; }
	.main-viewport { flex: 1; min-width: 0; background: #030712; display: flex; flex-direction: column; padding: 14px; box-sizing: border-box; gap: 12px; }
	.viewport-header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #1f2937; padding-bottom: 8px; flex-wrap: wrap; gap: 8px; }
	.viewport-header h2 { color: #f3f4f6; font-size: 1rem; margin: 0; }
	.bottom-metrics { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
	.metric-box { background: #0b0f19; border: 1px solid #1f2937; border-radius: 10px; padding: 8px 14px; }
	.metric-box span { font-size: 0.68rem; color: #9ca3af; text-transform: uppercase; }
	.metric-box h3 { margin: 2px 0 0 0; color: #fb923c; font-size: 1.35rem; font-family: monospace; }
	.green-text { color: #10b981 !important; }
	.cyan-text { color: #38bdf8 !important; }
	.split-middle { display: grid; grid-template-columns: 1.15fr 1fr; gap: 12px; flex-grow: 1; min-height: 0; }
	.canvas-container { background: #0b0f19; border: 1px solid #1f2937; border-radius: 12px; position: relative; overflow: hidden; }
	canvas { width: 100%; height: 100%; display: block; }
	.chart-panel { background: #0b0f19; border: 1px solid #1f2937; border-radius: 12px; padding: 12px; display: flex; flex-direction: column; gap: 8px; }
	.chart-panel h3 { margin: 0; font-size: 0.9rem; color: #e2e8f0; }
	.strategy-tags { display: flex; justify-content: space-between; font-size: 0.7rem; font-weight: bold; }
	.curve-svg { width: 100%; height: 150px; background: #050811; border-radius: 8px; border: 1px solid #1e293b; }
	.mayor-box { background: rgba(6, 78, 59, 0.3); border: 1px solid #059669; border-radius: 8px; padding: 10px; flex-grow: 1; overflow-y: auto; }
	.mayor-box h4 { margin: 0 0 4px 0; color: #34d399; font-size: 0.8rem; }
	.mayor-box p { margin: 0; color: #d1fae5; font-size: 0.75rem; line-height: 1.4; }
</style>