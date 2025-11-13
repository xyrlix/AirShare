FROM node:20-alpine AS deps
WORKDIR /app
COPY backend/package.json backend/tsconfig.json backend/ ./backend/
COPY frontend/package.json frontend/tsconfig.json frontend/vite.config.ts frontend/ ./frontend/
RUN cd /app/backend && npm install && cd /app/frontend && npm install

FROM node:20-alpine AS build
WORKDIR /app
COPY --from=deps /app /app
COPY backend/src ./backend/src
COPY frontend/src ./frontend/src
COPY frontend/index.html ./frontend/index.html
COPY frontend/public ./frontend/public
RUN cd /app/frontend && npm run build && cd /app/backend && npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=build /app/backend/package.json /app/backend/tsconfig.json /app/
COPY --from=build /app/backend/dist ./dist
COPY --from=build /app/frontend/dist ./frontend/dist
RUN npm install
ENV PORT=8443
EXPOSE 8443
CMD ["node", "dist/index.js"]
