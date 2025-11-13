<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
	xmlns:x="http://www.w3.org/1999/XSL/TransformAlias">
	<xsl:output method="xml" indent="yes"/>
	<xsl:namespace-alias stylesheet-prefix="x" result-prefix="xsl"/>
	<xsl:template match="/">
		<x:stylsheet version="3.0">
			<x:template match="/">
				<x:value-of select="/root/item"/>
			</x:template>
		</x:stylsheet>
	</xsl:template>
</xsl:stylesheet>