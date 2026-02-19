import typescript from '@rollup/plugin-typescript';
import resolve from '@rollup/plugin-node-resolve';
import postcss from 'rollup-plugin-postcss';
import copy from 'rollup-plugin-copy';
import replace from '@rollup/plugin-replace';
import prefixer from 'postcss-prefix-selector';

export default {
    input: 'src/Phileas.tsx',
    output: {
        file: 'dist/phileas.js',
        format: 'iife',
        sourcemap: true,
        name: 'PhileasEFB',
        globals: {
            '@microsoft/msfs-sdk': 'msfssdk',
        },
    },
    external: ['@microsoft/msfs-sdk'],
    plugins: [
        postcss({
            extract: 'phileas.css',
            minimize: true,
            use: ['sass'],
            plugins: [
                prefixer({
                    prefix: '.efb-view.phileas',
                    exclude: [],
                    transform(prefix, selector, prefixedSelector) {
                        if (selector.startsWith('body') || selector.startsWith('html')) {
                            return selector; // Don't prefix body/html if any
                        }
                        return prefixedSelector;
                    }
                })
            ]
        }),
        resolve(),
        typescript({
            tsconfig: './tsconfig.json'
        }),
        copy({
            targets: [
                { src: 'src/Assets/app-icon.svg', dest: 'dist/assets' },
                { src: 'src/Assets/icons/*', dest: 'dist/assets/icons' },
                { src: 'src/Assets/Fonts/*', dest: 'dist/assets/fonts' }
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
