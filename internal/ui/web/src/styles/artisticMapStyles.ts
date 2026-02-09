export const ARTISTIC_MAP_STYLES = {
    fonts: {
        city: {
            family: '"Pinyon Script", cursive',
            weight: 'bold',
            size: '24px',
            cssFont: "bold 24px 'Pinyon Script'"
        },
        town: {
            family: '"Pinyon Script", cursive',
            weight: 'normal',
            size: '22px',
            cssFont: "22px 'Pinyon Script'"
        },
        village: {
            family: '"Pinyon Script", cursive',
            weight: 'normal',
            size: '20px',
            cssFont: "20px 'Pinyon Script'"
        }
    },
    colors: {
        text: {
            active: '#0a0805',
            historical: '#3a2a1d',
        },
        icon: {
            gold: '#D4AF37',
            silver: '#C0C0C0',
            copper: '#B55A30',
            selected: '#8B1A4A',   // Victorian Garnet — currently narrating
            next: '#5C1234',       // Muted Garnet — next in queue
            stroke: '#0a0805'
        },
        shadows: {
            atmosphere: '0 0 1px rgba(10, 8, 5, 0.35), 0 0 2px rgba(10, 8, 5, 0.25)'
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
