export const ARTISTIC_MAP_STYLES = {
    colors: {
        text: {
            active: '#0a0805',
            historical: '#3a2a1d',
        },
        icon: {
            gold: '#D4AF37',
            silver: '#C0C0C0',
            copper: '#B55A30',
            historical: '#484848', // Cast Iron — visited/historic (Dark Grey to contrast with Silver)
            // Harmonized with Red Balloon (#E63946)
            // Complementary contrast for visibility
            selected: '#2A9D8F',     // Peacock Teal — Active (Complement to Red)
            next: '#E9C46A',         // Saffron Gold — Preparing
            selectedHalo: '#264653', // Deep Charcoal/Blue — Contrast glow
            nextHalo: '#F4A261',     // Sandy Orange — Warm glow
            normalHalo: '#f4ecd8',   // Paper color — Cutout effect
            stroke: '#0a0805'
        },
        shadows: {
            // Halo effect: Strong light stroke (paper color) to cut text out from background
            atmosphere: '-1px -1px 0 #f4ecd8, 1px -1px 0 #f4ecd8, -1px 1px 0 #f4ecd8, 1px 1px 0 #f4ecd8, 0 0 3px #f4ecd8, 0 0 1px rgba(10, 8, 5, 0.35)'
        }
    },
    tethers: {
        stroke: '#333',
        width: '0.5',
        opacity: 0.6,
        dotRadius: 1.5,
        dotOpacity: 0.8
    }
};
