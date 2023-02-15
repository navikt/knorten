const purgecss = require('@fullhuman/postcss-purgecss')
const cssnano = require('cssnano')
const postcssImport = require("postcss-import")

module.exports = {
    plugins: [
        postcssImport(),
        require('tailwindcss'),
        require('autoprefixer'),
        // todo: enable for prod, for now it's nice to keep it unminified
        // cssnano({
        //     preset: 'default'
        // }),
        purgecss({
            content: ["./templates/**/*.tmpl"],
            defaultExtractor: content => content.match(/[\w\-:.\/\[#%\]]+(?<!:)/g) || []
        }),
    ]
}