/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.tmpl"],
  theme: {
    fontFamily: {
      "font-family": "\"Source Sans Pro\", Arial, sans-serif"
    },
    extend: {},
  },
  plugins: [],
  presets: [require("@navikt/ds-tailwind")]
}
