// JSON Resume → Typst template
#let data = json("resume.json")

#set page(margin: (x: 1.2cm, y: 1.2cm), paper: "a4")
#set text(font: "New Computer Modern", size: 10pt)
#set par(justify: true)

#align(center)[
  #text(size: 18pt, weight: "bold")[#data.basics.name]
  #text(size: 10pt, fill: rgb("#555"))[
    #data.basics.location.city, #data.basics.location.countryCode •
    #link("mailto:" + data.basics.email)[#data.basics.email] •
    #link(data.basics.url)[#data.basics.url.replace("https://", "")]
    #for profile in data.basics.profiles [
      • #link(profile.url)[#lower(profile.network)]
    ]
  ]
]

#v(8pt)
#text(size: 9.5pt, fill: rgb("#333"))[#data.basics.summary]
#v(6pt)

#let section(title) = {
  v(8pt)
  text(size: 11pt, weight: "bold")[#title]
  v(-2pt)
  line(length: 100%, stroke: 0.5pt + rgb("#ccc"))
  v(4pt)
}

#section[Experience]
#for job in data.work [
  #grid(columns: (1fr, auto),
    text(weight: "semibold")[#job.position],
    text(size: 9pt, fill: rgb("#666"))[#job.startDate.slice(0, 4) — #if job.at("endDate", default: none) != none [#job.endDate.slice(0, 4)] else [Present]]
  )
  #text(size: 9pt, style: "italic", fill: rgb("#666"))[#job.name]
  #v(2pt)
  #for highlight in job.highlights [- #text(size: 9pt)[#highlight]]
  #v(6pt)
]

#section[Open Source & Projects]
#for project in data.projects [
  #text(weight: "semibold")[#project.name]
  #if project.at("url", default: none) != none [#text(size: 8pt, fill: rgb("#666"))[ — #link(project.url)[#project.url.replace("https://github.com/", "")]]]
  #linebreak()
  #text(size: 9pt)[#project.description]
  #v(4pt)
]

#section[Skills]
#for skill in data.skills [
  #grid(columns: (auto, 1fr), gutter: 8pt,
    text(weight: "semibold", size: 9pt)[#skill.name:],
    text(size: 9pt)[#skill.keywords.join(", ")]
  )
]

#section[Education]
#for edu in data.education [
  #grid(columns: (1fr, auto),
    text(weight: "semibold")[#edu.studyType of #edu.area],
    text(size: 9pt, fill: rgb("#666"))[#edu.startDate.slice(0, 4) — #edu.endDate.slice(0, 4)]
  )
  #text(size: 9pt, fill: rgb("#666"))[#edu.institution #if edu.at("score", default: none) != none [• #edu.score]]
  #v(4pt)
]

#if data.at("certificates", default: ()).len() > 0 [
  #section[Certifications]
  #for cert in data.certificates [
    #text(size: 9pt)[*#cert.name* — #cert.issuer (#cert.date.slice(0, 4))]
    #linebreak()
  ]
]
