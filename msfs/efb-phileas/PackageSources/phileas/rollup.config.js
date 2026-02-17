import typescript from '@rollup/plugin-typescript';
import resolve from '@rollup/plugin-node-resolve';
import postcss from 'rollup-plugin-postcss';
import copy from 'rollup-plugin-copy';
import replace from '@rollup/plugin-replace';

export default {
    input: 'src/Phileas.tsx',
    output: {
        dir: 'dist',
        format: 'es',
        sourcemap: true,
        entryFileNames: 'phileas.js',
    },
    external: ['@microsoft/msfs-sdk'],
    plugins: [
        postcss({
            extract: 'phileas.css',
            minimize: true,
            use: ['sass'],
        }),
        resolve(),
        typescript({
            tsconfig: './tsconfig.json'
        }),
        copy({
            targets: [
                { src: 'src/index.html', dest: 'dist' },
                { src: 'src/manifest.json', dest: 'dist' },
                { src: 'src/Assets/*', dest: 'dist/assets' }
            ]
        }),
        replace({
            preventAssignment: true,
            delimiters: ['', ''],
            BASE_URL: JSON.stringify('coui://html_ui/efb_ui/efb_apps/phileas'),
            VERSION: JSON.stringify(process.env.npm_package_version || '0.0.0'),
            BUILD_TIMESTAMP: JSON.stringify(new Date().toISOString()),
        }),
    ]
};
