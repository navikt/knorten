const purgecss = require('@fullhuman/postcss-purgecss')
const cssnano = require('cssnano')
const postcssImport = require("postcss-import")

module.exports = {
    plugins: [
        postcssImport(),
        require('tailwindcss'),
        require('autoprefixer'),
        cssnano({
             preset: 'default',
        }),
        // purgecss({
        //     content: ["./templates/**/*.tmpl"],
        //     defaultExtractor: content => content.match(/[\w\-:.\/\[#%\]]+(?<!:)/g) || []
        // }),
    ]
}
