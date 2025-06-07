<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<language>
			<xsl:for-each select="/root/language">
				<xsl:sort select="name" order="descending"/>
				<lang><xsl:value-of select="./@id"/></lang>
			</xsl:for-each>
		</language>
	</xsl:template>
</xsl:stylesheet>