#!/usr/bin/env node
import { calculate, Generations, Pokemon, Move, Field } from "@smogon/calc";
import { createServer } from "node:http";

const gen = Generations.get(9);
const port = parseInt(process.env.PORT || "8080", 10);

function runCalc(input) {
  const attacker = new Pokemon(gen, input.attacker.species, {
    nature: input.attacker.nature || "Adamant",
    evs: input.attacker.evs || {},
    item: input.attacker.item || undefined,
    ability: input.attacker.ability || undefined,
    boosts: input.attacker.boosts || {},
    level: input.attacker.level || 50,
  });

  const defender = new Pokemon(gen, input.defender.species, {
    nature: input.defender.nature || "Adamant",
    evs: input.defender.evs || {},
    item: input.defender.item || undefined,
    ability: input.defender.ability || undefined,
    boosts: input.defender.boosts || {},
    level: input.defender.level || 50,
  });

  const move = new Move(gen, input.move);

  const field = input.field
    ? new Field({
        weather: input.field.weather || undefined,
        terrain: input.field.terrain || undefined,
        isDoubles: input.field.isDoubles ?? true,
      })
    : new Field({ isDoubles: true });

  const result = calculate(gen, attacker, defender, move, field);
  const range = result.range();
  const maxHP = defender.maxHP();

  return {
    damage_min: range[0],
    damage_max: range[1],
    damage_min_pct: Math.round((range[0] / maxHP) * 1000) / 10,
    damage_max_pct: Math.round((range[1] / maxHP) * 1000) / 10,
    ko_chance: result.kpiDesc || null,
    description: result.fullDesc(""),
    defender_hp: maxHP,
  };
}

const server = createServer((req, res) => {
  if (req.method === "GET" && req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }

  if (req.method === "POST" && req.url === "/calc") {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      try {
        const input = JSON.parse(body);
        if (Array.isArray(input)) {
          const results = input.map((item) => {
            try {
              return runCalc(item);
            } catch (e) {
              return { error: e.message };
            }
          });
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(JSON.stringify(results));
        } else {
          const result = runCalc(input);
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(JSON.stringify(result));
        }
      } catch (e) {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: e.message }));
      }
    });
    return;
  }

  res.writeHead(404, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: "not found" }));
});

server.listen(port, () => {
  console.log(`vgc-calc listening on :${port}`);
});
